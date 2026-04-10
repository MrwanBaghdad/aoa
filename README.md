# aoa — Agent on Apple

> Run AI coding agents in isolated macOS VMs. No credential leaks. No lateral movement. No surprises.

`aoa` (Agent on Apple) is a local-first sandbox for running AI coding agents — Claude Code, opencode, Aider — inside hardware-isolated VMs on Apple Silicon. Each agent session gets its own kernel, its own filesystem, and controlled network egress. Your SSH keys, git credentials, and environment variables never enter the VM unless you explicitly allow them.

Built on [apple/container](https://github.com/apple/container), which uses macOS Virtualization.framework to give every container a separate VM — stronger isolation than shared-kernel containers, without the complexity of nested VMs.

---

## Why aoa?

Running agents in YOLO mode (full autonomy, no permission prompts) on bare metal is risky. The agent inherits everything: your SSH keys, AWS credentials, git tokens, `.env` files, and has unrestricted network access to your internal services.

| Tool | Isolation | macOS Native | Secrets | Open Source | Interactive Agent UX |
|------|-----------|-------------|---------|-------------|---------------------|
| **aoa** | VM (Virtualization.framework) | Yes | Keychain + SecretSpec + env | Yes | Yes |
| COI | Container (shared kernel) | Via Lima VM | Env vars only | Yes | Best in class |
| Docker sbx | MicroVM (libkrun) | Yes | Credential proxy | No | Built-in |
| E2B | MicroVM (Firecracker) | Cloud only | Env vars | Yes | SDK-based |
| microsandbox | MicroVM (libkrun) | Yes | "Keys never enter VM" | Yes | Programmatic |

`aoa`'s unique position: **native macOS VM isolation + declarative secrets management + interactive agent sessions + open source**.

---

## Requirements

- macOS 15+ on Apple Silicon (M1/M2/M3/M4)
- [apple/container](https://github.com/apple/container) installed
- [tmux](https://github.com/tmux/tmux) (`brew install tmux`)
- Go 1.23+ (to build from source)
- [SecretSpec](https://github.com/cachix/secretspec) _(optional — for 1Password / Keychain integration)_

---

## Installation

### Build from source

```bash
git clone https://github.com/MrwanBaghdad/aoa
cd aoa
go build -o aoa .
sudo mv aoa /usr/local/bin/
```

### Homebrew

Once a release is tagged, install via:

```bash
brew tap marwan/aoa https://github.com/MrwanBaghdad/aoa
brew install marwan/aoa/aoa
```

### Build the agent image

```bash
aoa build
```

This builds `aoa-agent:latest` using apple/container's BuildKit — an Ubuntu image with Claude Code and the `aoa-entrypoint` script pre-installed.

---

## Quick start

```bash
# Verify everything is set up correctly
aoa health

# Launch Claude Code in your project (current directory)
cd ~/myproject
aoa shell

# Use a specific project directory
aoa shell ~/myproject

# Open a plain bash shell instead of Claude Code
aoa shell --agent bash

# Run with no network restrictions (not recommended for production)
aoa shell --network open
```

When `aoa shell` starts, it:
1. Resolves credentials (see [Auth resolution order](#auth-resolution-order) below)
2. Writes them to a `0600` tmpfile
3. Spins up a VM with your project mounted at `/workspace`
4. Locks down network egress (blocks private networks and cloud metadata endpoints)
5. Mounts `.git/hooks` read-only (prevents supply-chain attacks)
6. Launches the agent inside tmux
7. Cleans up all secret tmpfiles on exit

---

## Secret management

### Auth resolution order

`aoa` tries credential sources in order, using the first that provides an LLM token:

1. **`secretspec.toml`** in your project directory (team vaults, 1Password, HashiCorp Vault)
2. **Env vars** listed in `~/.config/aoa/config.toml` (e.g. `ANTHROPIC_API_KEY` if already exported)
3. **macOS Keychain** — reads the OAuth token that Claude Code stores when you run `claude login`

This means **`aoa shell` works with zero configuration** if you've already authenticated with `claude` on your machine. No token generation, no copy-pasting keys.

```
$ aoa shell .
Auth: using Claude credentials from macOS Keychain
Starting aoa session a1b2c3d4 (slot 1) in ~/myproject
```

### Zero-config: already using Claude Code

If you use `claude` on your machine, you're done. `aoa` reads the same credentials from the Keychain (service: `Claude Code-credentials`) that Claude Code uses. The OAuth token is injected into the VM as `CLAUDE_CODE_OAUTH_TOKEN`.

### Explicit: environment variables

To use an API key directly, set it in your shell and it will be picked up:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
aoa shell .
```

Or declare which keys to inject in `~/.config/aoa/config.toml`:

```toml
[secrets]
env_keys = ["ANTHROPIC_API_KEY", "GITHUB_TOKEN"]
```

Only those specific keys are injected — nothing else from your host environment enters the VM.

### Declarative: secretspec.toml

For team projects, add a `secretspec.toml` to your repo root. `aoa` auto-detects it and uses it instead of the default resolution:

```toml
[project]
name = "my-agent-project"

[profiles.default]

[profiles.default.ANTHROPIC_API_KEY]
description = "Claude API key"
required = true

[profiles.default.GITHUB_TOKEN]
description = "For PR creation"
required = false

[profiles.default.DATABASE_URL]
description = "Dev database — written to a file, not env"
as_path = true
```

With `as_path = true`, the secret value is written to a tmpfile and its path is injected into the container — the plaintext value never appears in the process environment.

If the [SecretSpec CLI](https://github.com/cachix/secretspec) is installed, it resolves secrets from 1Password, macOS Keychain, HashiCorp Vault, or dotenv files. Without it, `aoa` falls back to env vars and declared defaults.

---

## Network policies

Three modes, configured globally or per-session:

### `restricted` (default)

Blocks private networks and cloud metadata endpoints. Allows internet HTTPS/DNS — the agent can call LLM APIs and install packages, but cannot reach your internal services.

```
Allowed:  0.0.0.0/0:443, 0.0.0.0/0:80, 0.0.0.0/0:53
Blocked:  10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
Blocked:  169.254.169.254 (AWS/GCP/Azure metadata)
Blocked:  100.100.100.200 (Alibaba metadata)
```

### `allowlist`

Default-deny with an explicit list of allowed IPs or CIDRs. For when you need tight control.

```toml
# config.toml
[network]
mode = "allowlist"
allowlist = ["203.0.113.5", "8.8.8.8/32"]
```

Or per-session:

```bash
aoa shell --network allowlist
```

### `open`

No restrictions. Use for debugging or when the agent needs to reach internal services.

```bash
aoa shell --network open
```

### Reaching ports on your Mac (`--allow-host`)

Each aoa session runs in a VM with its own network namespace. The host machine is accessible via the vmnet gateway IP, but `restricted` mode blocks it (along with all private ranges) to prevent lateral movement.

Use `--allow-host` to punch a precise hole to your Mac without opening up the rest of the private network:

```bash
# Allow the agent to reach any port on your Mac
aoa shell . --allow-host

# Restrict to specific ports only (repeatable)
aoa shell . --allow-host --allow-host-port 5432          # Postgres
aoa shell . --allow-host --allow-host-port 5432 --allow-host-port 6379  # + Redis
```

The gateway IP is detected inside the VM at runtime via `ip route` — you don't need to know or hardcode it. The ACCEPT rule is inserted before the private-network DROP rules so it works regardless of which subnet vmnet assigns.

> **Note:** `aoa` uses `iptables-legacy` inside the VM — not `nftables`. This is required because apple/container's default kernel does not include nftables modules.

---

## Session management

### Multiple parallel sessions (slots)

Run multiple agents on the same project simultaneously:

```bash
aoa shell --slot 1   # agent 1
aoa shell --slot 2   # agent 2 (new terminal)
```

Slots are scoped to the workspace directory. Auto-assignment picks the next available slot.

### Resume a session

```bash
aoa shell --resume         # resume slot 1
aoa shell --slot 2 --resume  # resume slot 2
```

Reattaches to the existing tmux session if the container is still running.

### List sessions

```bash
aoa list
```

```
ID          SLOT  STATUS   IMAGE             WORKSPACE              AGE
3995d919    1     running  aoa-agent:latest  ~/myproject            5m2s
a1b2c3d4    2     stopped  aoa-agent:latest  ~/myproject            1h3m
```

### Attach to a session

```bash
aoa attach 3995d919
```

### Persistent sessions

By default, containers are destroyed on exit. Use `--persistent` to keep the container alive:

```bash
aoa shell --persistent
```

Useful when you've installed tools inside the VM and want to reuse the environment across sessions.

---

## Security model

### What the VM can access

| Resource | In VM? | Notes |
|----------|--------|-------|
| `/workspace` (project dir) | Yes, read-write | The only host path mounted |
| `.git/hooks` | Yes, **read-only** | Prevents post-commit hook injection |
| `ANTHROPIC_API_KEY` | Yes | Injected via tmpfile, removed on exit |
| Other env vars | **No** | Host environment is not inherited |
| SSH keys (`~/.ssh`) | **No** | Never mounted |
| AWS/GCP credentials | **No** | Never mounted |
| Host filesystem (`/Users`) | **No** | VM has its own root |
| Host processes | **No** | Separate kernel |
| Host network stack | **No** | VM has its own network namespace |

### Supply-chain protection

The agent's git hooks directory is mounted read-only. A compromised dependency cannot install a malicious `post-commit` or `pre-push` hook that exfiltrates data on your next git operation — those hooks run on your host, not inside the VM.

### Secrets lifecycle

1. `aoa` resolves secrets before the container starts
2. Writes them to a `0600` tmpfile owned by your user
3. Mounts the tmpfile into the container as an env-file
4. Registers a cleanup handler (`trap` + signal handler) that removes the tmpfile on exit
5. The tmpfile is never written to the project directory

### Running the security test suite

Inside an active session:

```bash
aoa-test-security
```

Or from the host against a named container:

```bash
container exec <container-id> aoa-test-security
```

Expected output:

```
=== aoa security test suite ===

--- Credential Isolation ---
PASS  No host SSH keys (correctly blocked)
PASS  No host .env files (correctly blocked)
PASS  No password in env (correctly blocked)
PASS  No token in env (correctly blocked)
PASS  API key available

--- VM Isolation ---
PASS  No host filesystem (correctly blocked)
PASS  No host /home outside (correctly blocked)
PASS  Own kernel
PASS  Workspace mounted

--- Network Isolation (restricted mode) ---
PASS  Can reach HTTPS
PASS  Blocks private 192.x (correctly blocked)
PASS  Blocks private 10.x (correctly blocked)
PASS  Blocks metadata (correctly blocked)
PASS  DNS works

--- Supply Chain Protection ---
PASS  .git/hooks read-only (correctly blocked)

=== Results: 15 passed, 0 failed ===
```

---

## Configuration

Default config path: `~/.config/aoa/config.toml`

```toml
[sandbox]
image         = "aoa-agent:latest"
workspace_dir = "/workspace"
persistent    = false
max_slots     = 10
# extra_volumes = ["/path/to/tools:/tools:ro"]

[network]
mode      = "restricted"
# allowlist = ["203.0.113.5"]

[secrets]
provider  = "env"
env_keys  = ["ANTHROPIC_API_KEY", "GITHUB_TOKEN"]
```

### Per-session overrides

Most config values can be overridden at the command line:

```bash
aoa shell --image my-custom-agent:v2 --network open --persistent
```

---

## Architecture

```
macOS (Apple Silicon)
└── apple/container (Virtualization.framework — one VM per container)
    └── Ubuntu 24.04 with systemd (full OS inside the VM)
        ├── aoa-entrypoint (applies iptables rules, then exec's agent)
        ├── AI coding agent (Claude Code / opencode / bash)
        ├── /workspace (bind-mounted from host — only shared path)
        └── /run/secrets/ (tmpfiles — never persisted to disk in VM)
```

Each container is a **separate VM** with its own kernel, filesystem, and network namespace. This is hardware-level isolation via Apple's Virtualization.framework — not shared-kernel containers or user namespaces.

The `aoa-entrypoint` script runs as root inside the VM and applies the selected network policy using `iptables-legacy` before handing off to the agent. If the process running `aoa shell` is killed, the signal handler fires and removes secret tmpfiles from the host.

---

## Comparison with alternatives

### COI (Code on Incus)

The closest UX. COI uses Incus (LXC) system containers — shared kernel, so lighter weight but weaker isolation. On macOS it requires a Lima/Colima VM layer. `aoa` uses apple/container directly: one VM per agent, no intermediate layer, native macOS performance.

### Docker Sandboxes (sbx)

Proprietary Docker feature using libkrun MicroVMs. Good isolation but not open source, has rough edges (hard-coded CPU counts, broken env injection, clock drift after sleep). `aoa` is fully open source with a transparent security model.

### E2B

Cloud-based MicroVM sandboxes (Firecracker). Great for programmatic use but adds network latency, requires an account, and costs scale with usage. `aoa` is entirely local — no cloud dependency, no per-session cost.

### microsandbox

Strong isolation (libkrun) but designed for programmatic sandbox creation, not interactive agent sessions. The model is inverted — you embed it as a library, not run it as a CLI tool.

---

## Development

```bash
# Build
go build -o aoa .

# Go unit tests
make test-race

# Lint (golangci-lint)
make lint

# Vulnerability scan (govulncheck)
make vuln

# Python integration tests (CLI only, no container needed)
make integrations-cli

# All integration tests (requires apple/container)
make integrations

# Build + all fast tests (lint + vuln included)
make ci
```


### Project layout

```
aoa/
├── main.go
├── cmd/              # Cobra commands (shell, build, list, attach, health)
├── internal/
│   ├── config/       # TOML config parsing
│   ├── container/    # apple/container CLI wrapper, network policy
│   ├── secrets/      # Tmpfile injection, SecretSpec integration
│   ├── session/      # Slot manager, tmux lifecycle
│   └── security/     # Protected paths, JSONL audit log
├── images/           # Dockerfiles (base + agent)
├── scripts/          # entrypoint.sh, test-security.sh
├── config/           # default.toml, secretspec.toml example
└── tests/            # pytest integration tests
    ├── cli/          # Help, flags, arg validation
    ├── health/       # Dependency and env checks
    ├── session/      # List and attach
    ├── network/      # Network policy script assertions
    ├── secrets/      # Config structure, injection smoke tests
    └── security/     # Dockerfile constraints, protected paths, audit
```

---

## Roadmap

- [x] Phase 1 — Core orchestration (VM launch, secret injection, CLI)
- [x] Phase 2 — Security features (network policy, protected paths, audit log, test suite)
- [x] Phase 3 — Session management (slots, resume, persistent, tmux)
- [x] Phase 4 — SecretSpec integration (1Password, Keychain, Vault, as_path)
- [ ] Phase 5 — devcontainer.json compatibility
- [ ] Phase 6 — Monitoring & threat detection (traffic analysis, auto-pause/kill on HIGH/CRITICAL)

---

## License

Apache 2.0
