# Comparison with Other Agent Sandboxes

## At a glance

| | **aoa** | COI | Docker sbx | E2B | microsandbox |
|---|---|---|---|---|---|
| **Isolation** | VM (Virtualization.framework) | Container (shared kernel) | MicroVM (libkrun) | MicroVM (Firecracker) | MicroVM (libkrun) |
| **macOS native** | Yes | Via Lima VM | Yes | Cloud only | Yes |
| **Apple Silicon perf** | Native | Rosetta/QEMU overhead | Native | N/A | Native |
| **Secrets management** | SecretSpec (multi-provider) | Env vars only | Credential proxy | Env vars | "Keys never enter VM" |
| **Network filtering** | Yes (3 modes) | Yes (nft-based) | Yes | Yes | Unknown |
| **Open source** | Yes (Apache 2.0) | Yes (MIT) | No | Yes (Apache 2.0) | Yes (Apache 2.0) |
| **Interactive agent UX** | Yes | Best in class | Built-in (7 agents) | SDK-based | Programmatic |
| **devcontainer.json** | Planned (Phase 5) | No | No | No | No |
| **Local / offline** | Yes | Yes | Yes | No | Yes |
| **Cost** | Free | Free | Docker subscription | Per-session billing | Free |

---

## COI (Code on Incus)

**Best for:** Linux users who want the most polished interactive agent UX.

COI ([code-on-incus](https://github.com/mensfeld/code-on-incus)) is the closest tool in spirit. It wraps Incus (the community fork of LXD) to give agents isolated system containers. Its UX is excellent — slot-based sessions, persistent mode, tmux integration, network monitoring — and inspired several `aoa` features.

**Where aoa differs:**

- **Isolation depth:** Incus uses LXC (shared kernel). The agent and host share the same kernel — a kernel exploit or namespace escape reaches the host. `aoa` uses apple/container (Virtualization.framework): each container is a separate VM with its own kernel.
- **macOS story:** COI is designed for Linux. Running it on macOS requires Lima or Colima, which add a QEMU VM layer. `aoa` is native — `apple/container` runs directly on macOS with no intermediate VM.
- **Secrets management:** COI injects secrets as environment variables. `aoa` integrates with SecretSpec, supporting 1Password, macOS Keychain, HashiCorp Vault, and `as_path` (inject a tmpfile path, not the plaintext value).

**COI's edge:** More mature, more agents supported out of the box, better threat detection (Phase 6 of `aoa` is catching up).

---

## Docker Sandboxes (sbx)

**Best for:** Teams already on Docker with a subscription.

Docker's [AI Sandboxes](https://docs.docker.com/ai/sandboxes/) use libkrun to run MicroVMs, similar to aoa's isolation level. Built-in support for 7 agents (Claude Code, Cursor, Windsurf, etc.) with a credential proxy for injecting Docker tokens.

**Where aoa differs:**

- **Open source:** Docker sbx is a proprietary Docker feature. `aoa` is Apache 2.0.
- **Rough edges:** Hard-coded CPU/memory limits, broken env injection in early versions, clock drift after host sleep (libkrun limitation).
- **macOS performance:** libkrun has historically had perf issues on macOS Apple Silicon. `aoa` uses Apple's own Virtualization.framework, which is optimized for Apple Silicon.
- **Secrets:** Docker's model is a credential proxy. `aoa` uses SecretSpec with multi-provider support.

---

## E2B

**Best for:** Cloud-native workflows, programmatic sandbox creation, team sharing.

[E2B](https://e2b.dev) provides cloud MicroVM sandboxes (Firecracker) via an SDK and REST API. Excellent for building agentic products where sandboxes are created and destroyed programmatically.

**Where aoa differs:**

- **Local vs cloud:** E2B requires a network round-trip and an account. `aoa` is entirely local — works offline, no per-session cost, no data leaves your machine.
- **Interactive sessions:** E2B is SDK-first. `aoa` is designed for interactive terminal sessions.
- **Latency:** Cloud round-trips add latency for every file operation. `aoa` uses direct filesystem mounts — the agent sees your files at local disk speed.

**E2B's edge:** Better for products (you're building something that uses sandboxes), not for personal agent use. Team sharing, persistent environments, and browser automation support.

---

## microsandbox

**Best for:** Embedding sandbox creation in your own tool.

[microsandbox](https://github.com/microsandbox/microsandbox) uses libkrun for MicroVM isolation and is designed to be embedded as a library or daemon in other tools. Strong isolation properties ("agent's keys never enter the VM").

**Where aoa differs:**

- **Model:** microsandbox is a programmatic API, not a CLI for interactive sessions. You call it to create and manage sandboxes; `aoa` is what you run directly.
- **macOS:** libkrun on macOS Apple Silicon has known performance issues. `aoa` uses Virtualization.framework.
- **Secrets:** microsandbox's "keys never enter VM" model is similar to `aoa`'s but less flexible — no multi-provider fallback, no `as_path` support.

---

## Why apple/container?

`aoa` uses [apple/container](https://github.com/apple/container) as the runtime. The key property: **every container is a separate VM**.

This is different from Docker/Podman on macOS, where a single Linux VM is shared among all containers. With apple/container:

- Each `aoa shell` gets its own kernel, memory, and network namespace
- A compromise in one session cannot affect another
- macOS Virtualization.framework is purpose-built for Apple Silicon — startup is fast (typically under 2 seconds)
- OCI-compatible — pull from Docker Hub, GHCR, or any registry

The trade-off: heavier per-container overhead than shared-kernel containers. For interactive agent sessions (one or a handful running at a time), this is the right trade.
