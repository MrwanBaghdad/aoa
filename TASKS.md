# TASKS.md — Implementation Progress

## Decisions Log
- **CLI name:** `aoa` (Agent on apple/container) — working name matching the directory
- **Go module:** `github.com/marwan/aoa` — update when repo is created
- **Image strategy:** Two Dockerfiles — `Dockerfile.base` (systemd + tools) and `Dockerfile.agent` (adds Claude Code)
- **Secret injection:** tmpfile approach for Phase 1, SecretSpec CLI integration for Phase 4
- **Network policy:** iptables-legacy (as noted in plan — nftables not supported in default apple/container kernel)

---

## Phase 1 — Feasibility & Basic Orchestration

- [x] Initialize Go module and project directory structure
- [x] Create `images/Dockerfile.base` — Ubuntu + systemd + common tools
- [x] Create `images/Dockerfile.agent` — adds Claude Code on top of base
- [x] Create `scripts/entrypoint.sh` — network lockdown + agent launch (baked into image)
- [x] Implement `internal/config/config.go` — TOML config parsing
- [x] Implement `internal/container/runtime.go` — apple/container CLI wrapper
- [x] Implement `internal/secrets/inject.go` — tmpfile secret injection + cleanup
- [x] Implement `cmd/root.go` — Cobra root with version/flags
- [x] Implement `cmd/shell.go` — `aoa shell` launch command
- [x] Implement `cmd/build.go` — `aoa build` image build command
- [x] Create `config/default.toml` — default sandbox config
- [x] Create `main.go` entrypoint

## Phase 2 — Security Features

- [x] Implement `internal/container/network.go` — iptables rule generation (3 modes)
- [x] Implement `internal/security/paths.go` — protected path management
- [x] Implement `internal/security/audit.go` — JSONL audit logging
- [x] Create `scripts/test-security.sh` — security test suite
- [x] Implement `cmd/health.go` — `aoa health` security posture verification
- [x] Wire network policy into container run (entrypoint selects mode)

## Phase 3 — Session Management

- [x] Implement `internal/session/tmux.go` — tmux session lifecycle
- [x] Implement `internal/session/manager.go` — slot allocation, resume, state persistence
- [x] Implement `cmd/list.go` — `aoa list` show sessions
- [x] Implement `cmd/attach.go` — `aoa attach` reattach to session
- [x] Add `--slot`, `--resume`, `--persistent` flags to `aoa shell`

## Phase 4 — SecretSpec Integration

- [x] Implement `internal/secrets/secretspec.go` — SecretSpec CLI wrapper
- [x] Create `config/secretspec.toml` example
- [x] Update `cmd/shell.go` to auto-detect and use secretspec.toml
- [x] Support `as_path` secrets (write to tmpfile, inject path)
- [x] Multi-provider fallback (pass through to SecretSpec)

---

## Phases 5–6 (future)
- Phase 5: devcontainer.json compatibility
- Phase 6: Monitoring & threat detection
