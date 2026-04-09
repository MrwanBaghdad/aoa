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
ci: build test-race integrations-cli

clean:
	rm -f $(BINARY) coverage.out coverage.html
