.PHONY: build test test-race test-coverage lint integrations integrations-cli integrations-setup clean

BINARY   := aoa
GO       := go
PYTEST   := pytest
PYTHON   := python3

# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------

build:
	$(GO) build -o $(BINARY) .

build-release:
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY) .

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-coverage:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

# ---------------------------------------------------------------------------
# Python integration tests
# ---------------------------------------------------------------------------

integrations-setup:
	$(PYTHON) -m pip install -r tests/support/requirements.txt

# All integration tests (requires apple/container)
integrations:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/ -v

# CLI-only tests — no container runtime required, fast
integrations-cli:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/ -v -m cli

# Specific test groups (mirror CI parallelism)
integrations-health:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/health/ -v

integrations-session:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/session/ -v

integrations-network:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/network/ -v

integrations-security:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/security/ -v

integrations-secrets:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/secrets/ -v

integrations-debug:
	AOA_BINARY=$(PWD)/$(BINARY) $(PYTEST) tests/ -v -s

lint-python:
	ruff check tests/
	ruff format --check tests/

# ---------------------------------------------------------------------------
# Convenience targets
# ---------------------------------------------------------------------------

# Run everything: build, Go tests, CLI integration tests
ci: build test-race lint vuln integrations-cli

# Scan the built image for CRITICAL/HIGH CVEs (requires aoa build first)
scan-image:
	@IMAGE=aoa-agent:latest; \
	TARBALL=$$(mktemp /tmp/aoa-scan-XXXXXX.tar); \
	echo "Exporting $$IMAGE..."; \
	container image save $$IMAGE --output $$TARBALL; \
	echo "Scanning with Trivy..."; \
	trivy image --input $$TARBALL --severity CRITICAL,HIGH --exit-code 1 \
	  --ignorefile .trivyignore --no-progress; \
	STATUS=$$?; rm -f $$TARBALL; exit $$STATUS

clean:
	rm -f $(BINARY) coverage.out coverage.html

# ---------------------------------------------------------------------------
# Release / packaging
# ---------------------------------------------------------------------------

# Usage: make formula VERSION=0.1.0
# Fetches the release tarball, computes sha256, updates Formula/aoa.rb.
# Copy the result to your homebrew-tap repo before pushing.
formula:
	@test -n "$(VERSION)" || (echo "usage: make formula VERSION=x.y.z" && exit 1)
	@URL="https://github.com/MrwanBaghdad/aoa/archive/refs/tags/v$(VERSION).tar.gz"; \
	 SHA=$$(curl -sL "$$URL" | shasum -a 256 | awk '{print $$1}'); \
	 sed -i '' \
	     -e "s|head \".*\"|head \"https://github.com/MrwanBaghdad/aoa.git\", branch: \"main\"\n\n  version \"$(VERSION)\"\n  url \"$$URL\"\n  sha256 \"$$SHA\"|" \
	     Formula/aoa.rb; \
	 echo "Formula/aoa.rb updated for v$(VERSION) (sha256=$$SHA)"

# Verify the Nix flake builds (requires Nix with flakes enabled)
nix-build:
	nix build .#
