# Security Model

This document describes the threat model `aoa` is designed to defend against, the mechanisms it uses, and the known limitations.

## Threat model

`aoa` assumes that the AI coding agent **cannot be fully trusted**. Either the agent itself, or a dependency it installs, or a package it executes, may attempt to:

- Exfiltrate secrets from the host environment
- Access credentials stored on disk (SSH keys, AWS config, git tokens)
- Reach internal services on your local network
- Modify git hooks to persist malicious code beyond the session
- Probe cloud metadata endpoints (169.254.169.254) to harvest credentials
- Establish reverse shells or C2 connections

`aoa` defends against all of these by default.

---

## Defense layers

### 1. VM isolation (Virtualization.framework)

Each `aoa shell` invocation creates a new VM via [apple/container](https://github.com/apple/container), which uses macOS Virtualization.framework. The VM has:

- Its own kernel (not shared with the host)
- Its own filesystem (not the host filesystem)
- Its own process table
- Its own network namespace

This is hardware-level isolation. The agent cannot escape the VM through kernel exploits, `/proc` tricks, or namespace breakouts — unlike shared-kernel containers (Docker, Podman, Incus/LXC).

### 2. Credential isolation

The VM starts with a clean environment. The host environment is **never** inherited:

| What the agent can see | What it cannot see |
|------------------------|-------------------|
| `ANTHROPIC_API_KEY` (explicitly injected) | All other host env vars |
| `/workspace` (project directory) | `~/.ssh/` |
| `/run/secrets/` (as_path tmpfiles) | `~/.aws/`, `~/.config/gcloud/` |
| — | `~/.gitconfig` credentials |
| — | Host `.env` files |
| — | macOS Keychain |

The only secrets that enter the VM are those you explicitly declare — either in `env_keys` in your config or in `secretspec.toml`.

**Secrets lifecycle:**

1. Resolved on the host before the container starts
2. Written to a `0600` tmpfile in `/tmp` (owned by your user)
3. Passed to the container as an env-file or volume mount
4. A `signal.Notify` handler and deferred `bundle.Cleanup()` ensure the tmpfile is deleted when `aoa` exits — even on SIGINT/SIGTERM
5. The tmpfile is never written inside the project directory

### 3. Network egress filtering

The `aoa-entrypoint` script applies `iptables-legacy` rules inside the VM before handing off to the agent. In `restricted` mode (default):

```
Policy: OUTPUT DROP (default deny)

Allowed:
  loopback (lo)
  UDP/TCP port 53 (DNS)
  TCP port 443 (HTTPS)
  TCP port 80 (HTTP)
  ESTABLISHED,RELATED (responses to allowed connections)

Blocked:
  10.0.0.0/8        (RFC 1918 — private networks)
  172.16.0.0/12     (RFC 1918)
  192.168.0.0/16    (RFC 1918)
  169.254.169.254   (AWS/GCP/Azure instance metadata)
  100.100.100.200   (Alibaba Cloud instance metadata)
```

This prevents:
- Lateral movement to internal services (databases, Kubernetes clusters, internal APIs)
- Cloud metadata credential harvesting (SSRF via metadata endpoint)
- Reaching host services bound to `localhost` or local network interfaces

> **Implementation note:** The VM kernel in apple/container does not include nftables modules, so `iptables-legacy` is used. The base image (`Dockerfile.base`) installs the `iptables-legacy` package and sets it as the system default via `update-alternatives`.

### 4. Supply-chain attack protection

The `.git/hooks` directory is mounted **read-only** inside the VM:

```
Host: /Users/you/myproject/.git/hooks  →  Container: /workspace/.git/hooks (ro)
```

A compromised dependency cannot install a `post-commit`, `pre-push`, or `prepare-commit-msg` hook. Git hooks run on the host when you `git commit` or `git push` — if the agent could write to `.git/hooks`, it could persist malicious code that executes outside the sandbox on your next git operation.

### 5. Ephemeral by default

Containers are destroyed on exit (`--rm` equivalent). The agent cannot persist state to the VM filesystem. Between sessions, only the project directory (`/workspace`) survives — and that's your code repository, which is visible to `git diff` and code review.

Use `--persistent` deliberately: it trades the ephemeral security property for convenience (cached tool installs, etc.).

---

## Security test suite

Run `aoa-test-security` inside a session to verify the security properties:

```bash
aoa shell --agent bash
# inside the container:
aoa-test-security
```

The test suite checks:

| Test | What it verifies |
|------|-----------------|
| No host SSH keys | `~/.ssh/id_*` does not exist |
| No host .env files | No `.env` outside `/workspace` |
| No password in env | `env` contains no `password` |
| No token in env | No `GITHUB_TOKEN`, `NPM_TOKEN`, etc. |
| API key available | `ANTHROPIC_API_KEY` was injected correctly |
| No host filesystem | `/Users` does not exist |
| No host /home | `/home/<username>` does not exist |
| Own kernel | `/proc/version` exists (VM has its own kernel) |
| Workspace mounted | `/workspace` exists and is readable |
| HTTPS reachable | `curl https://api.anthropic.com` succeeds |
| 192.168.x blocked | `curl http://192.168.1.1` times out |
| 10.x blocked | `curl http://10.0.0.1` times out |
| Metadata blocked | `curl http://169.254.169.254` times out |
| DNS works | `nslookup api.anthropic.com` resolves |
| .git/hooks read-only | `touch /workspace/.git/hooks/x` fails |

---

## Known limitations

### Memory not returned to host

Virtualization.framework does not return memory freed inside the VM to the host. For memory-intensive workloads (e.g., compiling large projects), you may need to restart the container periodically.

### IPv6 gaps

The default kernel in apple/container has some IPv6 gaps. `aoa`'s network policy uses IPv4 rules only. If the agent resolves a host to an IPv6 address, egress filtering may not apply.

### Root inside the VM

The agent process runs as root inside the VM by default. This is required to apply `iptables` rules at startup. The host is protected by VM isolation — root inside the VM does not grant elevated privileges on the host.

### iptables rules require root

The `aoa-entrypoint` script logs a warning and skips network policy if it detects it is not running as root. In practice, apple/container runs containers as root, so this is not an issue.

### Persistent sessions

`--persistent` mode keeps the container alive after the session exits. A persistent container retains its filesystem state, including any packages or tools installed by the agent. The same credential isolation and network policies apply on resume, but the ephemeral property is traded for convenience.

---

## Audit log

`aoa` writes a JSONL audit log for each session at:

```
~/.local/share/aoa/audit/session-<id>.jsonl
```

Each line is a JSON object:

```json
{
  "timestamp": "2026-04-09T10:15:30Z",
  "session_id": "3995d919-...",
  "container_id": "aoa-3995d919",
  "severity": "INFO",
  "event": "session_started",
  "details": { "workspace": "/Users/you/myproject", "network_mode": "restricted" }
}
```

Severity levels: `INFO`, `LOW`, `MEDIUM`, `HIGH`, `CRITICAL`.

Phase 6 will add real-time traffic monitoring, pattern detection (reverse shells, DNS tunneling, data exfiltration), and automated response (pause on HIGH, kill on CRITICAL).
