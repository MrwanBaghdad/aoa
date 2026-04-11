# PLAN.md — Agent Sandbox for macOS (apple/container)

## What We're Building

A local-first, security-first agent sandbox tool for macOS Apple Silicon that runs AI coding agents (Claude Code, opencode, Aider, etc.) in isolated environments using `apple/container` (Apple's open-source container runtime). Each agent gets its own lightweight VM with hardware-level isolation via macOS Virtualization.framework.

**Working name:** TBD (the CLI command, like `coi` is for Code on Incus)

## Why This Exists

Running coding agents in YOLO mode (full autonomy, no permission prompts) is dangerous on bare metal — the agent inherits your SSH keys, env vars, git credentials, API tokens, and has full network access. Existing solutions have tradeoffs:

- **COI (Code on Incus):** Best UX for this use case but uses shared-kernel system containers (not VMs). On macOS requires Lima/Colima VM layer — two indirections. No secrets management integration.
- **Docker Sandboxes (sbx):** MicroVM isolation via libkrun but proprietary, experimental, rough edges (hard-coded CPU, broken env injection, clock drift on sleep).
- **E2B:** Cloud-only, adds network latency, ephemeral, costs scale.
- **microsandbox:** Good isolation (libkrun) but inverted model — designed for programmatic sandbox creation, not interactive agent sessions.

None of them combine: native macOS runtime + VM-per-agent isolation + proper secrets management + devcontainer spec compatibility.

## Architecture

```
macOS (Apple Silicon)
└── apple/container (Virtualization.framework — one VM per container)
    └── Ubuntu/Fedora with systemd (full OS inside the VM)
        ├── AI coding agent (Claude Code / opencode / Aider)
        ├── Podman (optional, for when agent needs services)
        │   ├── postgres
        │   └── redis
        ├── iptables rules (network egress filtering)
        └── /workspace (mounted from host, only thing shared)
```

Key architectural decisions:
- **apple/container runs each container as a separate VM** — own kernel, own filesystem, own network. This is stronger isolation than COI (shared kernel) without the complexity of nested VMs.
- **systemd works inside apple/container** — use a Fedora/Ubuntu image with systemd as entrypoint. This gives a full OS environment where Podman/Docker can run natively (confirmed by Podman community demos).
- **Caveat:** Use iptables-legacy not nftables inside the VM (default kernel missing nftables features). IPv6 networking has some gaps.
- **Secrets never baked into images** — injected at runtime via tmpfiles or env, cleaned up on exit. SecretSpec integration for multi-provider support.

## Technology Stack

- **Runtime:** apple/container (https://github.com/apple/container) — Apache 2.0, requires Apple Silicon, best on macOS 26+
- **Secrets:** SecretSpec (https://github.com/cachix/secretspec) — standalone CLI, supports 1Password, Keychain, Vault, env, dotenv
- **Spec:** devcontainer.json — portable, ecosystem-compatible, declares secrets, features, mounts
- **Language:** Go (same as COI — fast, single binary, cobra for CLI, good cross-platform story)
- **Packaging:** Homebrew tap for the CLI binary + non-Go dependencies (SecretSpec, apple/container CLI if not already installed)
- **Distribution:** `brew tap yourname/sandbox && brew install sandbox`

## Competitive Landscape Context

| Tool | Isolation | macOS Native | Secrets | Open Source | Interactive Agent UX |
|------|-----------|-------------|---------|-------------|---------------------|
| **This project** | VM (Virtualization.framework) | Yes | SecretSpec | Yes | Building it |
| COI | Container (shared kernel) | Via Lima VM | Env vars only | Yes | Best in class |
| Docker sbx | MicroVM (libkrun) | Yes | Credential proxy | No | Built-in for 7 agents |
| E2B | MicroVM (Firecracker) | Cloud only | Env vars | Yes (Apache 2.0) | SDK-based |
| microsandbox | MicroVM (libkrun) | Yes | "Keys never enter VM" | Yes (Apache 2.0) | Programmatic, not interactive |

Our unique position: only tool combining native macOS VM isolation + proper secrets management + interactive agent sessions + open source.

## Phase 1 — Feasibility & Basic Orchestration

**Goal:** Prove that a coding agent works inside apple/container with mounted workspace and injected credentials.

### Tasks

1. **Verify apple/container basics**
   ```bash
   container run -it --volume $(pwd):/workspace ubuntu:24.04 /bin/bash
   ```
   Confirm: volume mounting works, interactive terminal works, can install packages.

2. **Build an agent base image**
   Create a Dockerfile with:
   - Ubuntu or Fedora base
   - systemd as entrypoint (for full OS behavior)
   - Common dev tools (git, curl, build-essential, node, python)
   - Claude Code or opencode pre-installed
   - tmux for session management
   ```dockerfile
   FROM fedora
   RUN dnf install -y systemd git curl tmux nodejs python3 pip
   # Install Claude Code
   RUN npm install -g @anthropic-ai/claude-code
   ENTRYPOINT ["/usr/lib/systemd/systemd"]
   STOPSIGNAL SIGRTMIN+3
   ```
   Build with: `container build -t agent-sandbox .`

3. **Credential injection**
   ```bash
   # Create tmpfile with secrets, mount read-only, cleanup on exit
   SECRET_FILE=$(mktemp)
   echo "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}" > "$SECRET_FILE"
   trap "rm -f $SECRET_FILE" EXIT

   container run -it \
     --volume $(pwd):/workspace \
     --volume "${SECRET_FILE}:/run/secrets/env:ro" \
     --env-file "$SECRET_FILE" \
     agent-sandbox
   ```

4. **Wrap in a launcher script**
   ```bash
   #!/bin/bash
   # ac-sandbox — v0.1
   PROJECT_DIR="${1:-.}"
   IMAGE="agent-sandbox:latest"
   # ... secret injection, container run, cleanup
   ```

### Definition of Done
- Agent launches inside apple/container
- Can read/edit files in /workspace
- Changes persist on host filesystem
- API key works (agent can call LLM)
- Host env vars, SSH keys, git credentials are NOT visible inside container

## Phase 2 — Security Features

**Goal:** Prove the security properties with a test suite.

### Security Features to Implement

#### 2a. Credential Isolation
- Don't pass host environment (default with apple/container)
- Mount only the workspace directory
- Inject only the LLM API key (via tmpfile, cleaned up on exit)
- No host SSH keys, no .env files, no git credentials inside container

#### 2b. Network Egress Filtering
Entrypoint script that locks down networking before agent starts:
```bash
# Default deny outbound
iptables -P OUTPUT DROP
# Allow loopback
iptables -A OUTPUT -o lo -j ACCEPT
# Allow DNS
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
# Allow HTTPS (for LLM API calls)
iptables -A OUTPUT -p tcp --dport 443 -j ACCEPT
# Block private networks (prevent lateral movement)
iptables -A OUTPUT -d 10.0.0.0/8 -j DROP
iptables -A OUTPUT -d 172.16.0.0/12 -j DROP
iptables -A OUTPUT -d 192.168.0.0/16 -j DROP
# Block cloud metadata endpoints
iptables -A OUTPUT -d 169.254.169.254 -j DROP
# Allow established connections
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
```
NOTE: Must use `iptables-legacy` not `nftables` inside apple/container VMs.

Three modes (configurable):
- **restricted** — block private networks, allow internet
- **allowlist** — only approved domains/IPs
- **open** — unrestricted

#### 2c. Protected Paths
- Mount `.git/hooks` as read-only (prevent supply-chain attacks via post-commit hooks)
- Mount other security-sensitive paths read-only
- Configurable via config file

#### 2d. VM Isolation Verification
- Confirm separate kernel (`uname -r` shows VM kernel)
- Confirm no host filesystem access (`/Users/` doesn't exist)
- Confirm no host process visibility (`ps aux` shows only container processes)
- Confirm no host network access (can't reach host services unless explicitly allowed)

### Security Test Suite
```bash
#!/bin/bash
# test-security.sh — run inside the container
PASS=0; FAIL=0

test_case() {
  local name="$1" cmd="$2" expect_fail="$3"
  if eval "$cmd" > /dev/null 2>&1; then
    if [ "$expect_fail" = "true" ]; then
      echo "FAIL: $name (should have been blocked)"; FAIL=$((FAIL+1))
    else
      echo "PASS: $name"; PASS=$((PASS+1))
    fi
  else
    if [ "$expect_fail" = "true" ]; then
      echo "PASS: $name (correctly blocked)"; PASS=$((PASS+1))
    else
      echo "FAIL: $name (should have worked)"; FAIL=$((FAIL+1))
    fi
  fi
}

# Credential isolation
test_case "No host SSH keys"       "ls ~/.ssh/id_rsa"            "true"
test_case "No host .env files"     "find / -name '.env' 2>/dev/null | grep -v workspace | grep -q ." "true"
test_case "No host env leak"       "env | grep -qi 'password\|token\|secret' | grep -v ANTHROPIC" "true"
test_case "API key available"      "test -n \"\$ANTHROPIC_API_KEY\"" "false"

# Network isolation (after iptables setup)
test_case "Can reach HTTPS"        "curl -s --max-time 5 https://api.anthropic.com" "false"
test_case "Blocks private net"     "curl -s --max-time 3 http://192.168.1.1"        "true"
test_case "Blocks metadata"        "curl -s --max-time 3 http://169.254.169.254"    "true"

# VM isolation
test_case "No host filesystem"     "ls /Users"                    "true"
test_case "Own kernel"             "test -f /proc/version"        "false"

echo "Results: $PASS passed, $FAIL failed"
```

### Definition of Done
- Test suite passes all cases
- Network filtering blocks unauthorized egress
- Credential isolation prevents host secret leakage
- VM boundary prevents filesystem/process escape

## Phase 3 — Session Management

**Goal:** Multi-session support with persistence and resume.

### Features
- **tmux wrapping** — all agent sessions run in tmux for detach/reattach
- **Slot allocation** — `sandbox shell --slot 1`, `sandbox shell --slot 2` for parallel agents
- **Session resume** — `sandbox shell --resume` picks up where you left off
- **Workspace scoping** — sessions are tied to the project directory
- **Ephemeral by default** — container destroyed on exit, workspace persists
- **Persistent mode** — `sandbox shell --persistent` keeps container and installed tools between sessions

## Phase 4 — SecretSpec Integration

**Goal:** Multi-provider secrets management.

### Features
- Parse `secretspec.toml` for secret declarations
- Resolve secrets from configured providers (1Password, Keychain, env, Vault)
- Multi-provider fallback (try team vault first, then local keychain)
- Per-environment profiles (development defaults, production strict)
- `as_path` support — write secrets to tmpfiles, pass paths not values
- Cleanup on container exit

### Example Flow
```toml
# secretspec.toml
[project]
name = "my-agent-project"

[profiles.default]
ANTHROPIC_API_KEY = { description = "Claude API key", required = true }
GITHUB_TOKEN = { description = "For PR creation", required = false }
DATABASE_URL = { description = "Dev database", as_path = true }

[profiles.development]
ANTHROPIC_API_KEY = { default = "sk-ant-..." }
DATABASE_URL = { default = "postgres://localhost:5432/dev" }
```

The launcher resolves these before starting the container and injects them appropriately.

## Phase 5 — devcontainer.json Compatibility

**Goal:** Parse devcontainer.json and use it to configure the sandbox.

### Features
- Read image, features, mounts, env vars, secrets from devcontainer.json
- Map devcontainer features to apple/container configuration
- Secret declarations from devcontainer.json fed to SecretSpec
- Portable — same devcontainer.json works in VS Code, Codespaces, and our tool

## Phase 6 — Monitoring & Threat Detection

**Goal:** Real-time traffic analysis and automated response (COI's differentiating feature, replicated).

### Features
- iptables/nftables traffic logging inside the VM
- Pattern detection: reverse shells, C2 connections, data exfiltration, DNS tunneling, credential scanning
- Severity classification (LOW/MEDIUM/HIGH/CRITICAL)
- Auto-pause on HIGH threats
- Auto-kill on CRITICAL threats
- Audit logs in JSONL format
- `sandbox health` command for system verification

## Key Technical Notes

### apple/container specifics
- Each container is a separate VM via Virtualization.framework
- OCI-compatible images — pull from Docker Hub, GHCR, any registry
- BuildKit support via container-builder-shim
- Per-container IP addresses on macOS 26 (no port mapping needed)
- Rosetta support for running x86_64 containers on ARM
- Memory freed inside containers not returned to host (Virtualization.framework limitation) — may need periodic restarts for memory-intensive workloads
- `container run -it --volume host:container image` is the basic invocation

### Running systemd inside apple/container
```dockerfile
FROM fedora
RUN dnf install -y systemd
ENTRYPOINT ["/usr/lib/systemd/systemd"]
STOPSIGNAL SIGRTMIN+3
```
```bash
cid=$(container run -d fedora-systemd)
container exec -it $cid bash
```
This gives a full OS with package manager, systemd services, and Podman/Docker support inside.

### Networking constraints
- Use `iptables-legacy` not `nftables` (default kernel missing nftables features)
- IPv6 has some gaps in the default kernel
- Install `iptables-legacy` package and set as default

### Compose / multi-container
- No native compose in apple/container yet
- Community projects: container-compose (Rust, alpha), Container-Compose (Swift, PR to upstream)
- Socktainer provides Docker-compatible socket for using standard docker-compose with apple/container
- For agent use case: run services (Postgres, Redis) as separate apple/container instances, or run Podman inside the agent VM

### devenv/nix2container (context from prior work)
- devenv uses nix2container to build OCI images — does NOT use Docker/BuildKit
- nix2container produces a JSON spec, needs patched skopeo (`skopeo-nix2container`) with `nix:` transport to copy
- On macOS, building containers requires a Linux builder (QEMU VM via nix-darwin, or Determinate Nix native builder)
- We successfully got devenv images into apple/container by: building with devenv → copying with nix2container's skopeo to OCI layout or local registry → running with `container run`
- This path works but is complex — for this product, prefer standard Dockerfiles with apple/container's native BuildKit

## File Structure (Proposed)

```
sandbox/
├── PLAN.md                # This file
├── README.md              # User-facing documentation
├── go.mod                 # Go module
├── go.sum
├── main.go                # Entrypoint
├── cmd/
│   ├── root.go            # Cobra root command
│   ├── shell.go           # sandbox shell — launch agent session
│   ├── build.go           # sandbox build — build agent image
│   ├── list.go            # sandbox list — show sessions
│   ├── health.go          # sandbox health — verify security posture
│   └── attach.go          # sandbox attach — reattach to session
├── internal/
│   ├── container/
│   │   ├── runtime.go     # apple/container CLI wrapper
│   │   ├── image.go       # Image build/pull management
│   │   └── network.go     # Network policy setup (iptables rules)
│   ├── secrets/
│   │   ├── inject.go      # Tmpfile creation, cleanup, mounting
│   │   └── secretspec.go  # SecretSpec CLI integration
│   ├── session/
│   │   ├── manager.go     # Slot allocation, resume, lifecycle
│   │   └── tmux.go        # tmux wrapping for detach/reattach
│   ├── security/
│   │   ├── monitor.go     # Traffic analysis, threat detection (Phase 6)
│   │   ├── audit.go       # JSONL audit logging
│   │   └── paths.go       # Protected path management
│   └── config/
│       ├── config.go      # TOML config parsing
│       └── devcontainer.go # devcontainer.json parsing (Phase 5)
├── images/
│   ├── Dockerfile.base    # Base image with systemd + common tools
│   └── Dockerfile.agent   # Agent-specific (Claude Code, opencode, etc.)
├── scripts/
│   ├── entrypoint.sh      # Network lockdown + agent launch (baked into image)
│   └── test-security.sh   # Security test suite
├── config/
│   ├── default.toml       # Default sandbox configuration
│   └── secretspec.toml    # Example SecretSpec configuration
├── Formula/
│   └── sandbox.rb         # Homebrew formula
└── docs/
    ├── security.md        # Security model documentation
    └── comparison.md      # Comparison with COI, sbx, E2B, etc.
```

### Homebrew Distribution

```ruby
# Formula/sandbox.rb
class Sandbox < Formula
  desc "Secure agent sandbox for macOS using apple/container"
  homepage "https://github.com/yourname/sandbox"
  url "https://github.com/yourname/sandbox/archive/refs/tags/v0.1.0.tar.gz"
  license "Apache-2.0"

  depends_on "go" => :build
  depends_on "apple/container"  # or however it's tapped
  depends_on "secretspec"       # SecretSpec binary

  def install
    system "go", "build", "-o", bin/"sandbox", "."
  end
end
```

### Key Go Dependencies

```go
// go.mod
module github.com/yourname/sandbox

go 1.24

require (
    github.com/BurntSushi/toml v1.6.0   // Config parsing
    github.com/spf13/cobra v1.10.2       // CLI framework (same as COI)
    github.com/google/uuid v1.6.0        // Session IDs
)

## References

- apple/container: https://github.com/apple/container
- apple/containerization: https://github.com/apple/containerization
- COI (Code on Incus): https://github.com/mensfeld/code-on-incus
- Docker Sandboxes: https://docs.docker.com/ai/sandboxes/
- SecretSpec: https://devenv.sh/blog/2025/07/21/announcing-secretspec-declarative-secrets-management/
- devcontainer spec: https://github.com/devcontainers/spec
- Socktainer (Docker API for apple/container): https://github.com/socktainer/socktainer
- container-compose: https://github.com/noghartt/container-compose
- Podman inside apple/container: https://github.com/containers/podman/discussions/27278
- libkrun (VMM library): https://github.com/containers/libkrun
- microsandbox: https://github.com/microsandbox/microsandbox
- awesome-sandbox: https://github.com/restyler/awesome-sandbox
