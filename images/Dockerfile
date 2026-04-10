# aoa agent image — multi-stage build
# Stage 1 (base): Ubuntu 24.04 + systemd + dev tools + iptables-legacy
# Stage 2 (agent): adds Claude Code and aoa scripts

# ── base ──────────────────────────────────────────────────────────────────────
FROM ubuntu:24.04 AS base

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    systemd \
    systemd-sysv \
    dbus \
    curl \
    wget \
    git \
    tmux \
    build-essential \
    python3 \
    python3-pip \
    python3-venv \
    nodejs \
    npm \
    jq \
    unzip \
    ca-certificates \
    gnupg \
    lsb-release \
    iptables \
    iproute2 \
    iputils-ping \
    dnsutils \
    libcap2-bin \
    && rm -rf /var/lib/apt/lists/*

# Set iptables-legacy as default (nftables not supported in apple/container VMs)
RUN update-alternatives --set iptables /usr/sbin/iptables-legacy \
    && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy

# Disable unnecessary systemd units for container use
RUN systemctl mask \
    getty@tty1.service \
    systemd-logind.service \
    apt-daily.service \
    apt-daily-upgrade.service \
    apt-daily.timer \
    apt-daily-upgrade.timer

RUN mkdir -p /workspace

# Create non-root agent user. UID is adjusted at runtime to match the
# workspace mount owner (see entrypoint.sh) so the agent can write files
# that appear correctly owned on the host filesystem.
RUN useradd --create-home --shell /bin/bash agent

# ── agent ─────────────────────────────────────────────────────────────────────
FROM base AS agent

# Install Node.js LTS (for Claude Code) via NodeSource
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Copy the network lockdown + agent entrypoint script
COPY scripts/entrypoint.sh /usr/local/bin/aoa-entrypoint
RUN chmod +x /usr/local/bin/aoa-entrypoint

# Install the security test suite
COPY scripts/test-security.sh /usr/local/bin/aoa-test-security
RUN chmod +x /usr/local/bin/aoa-test-security

WORKDIR /workspace

# aoa-entrypoint applies network policy then exec's the given command.
# Default command is bash — override at runtime for the agent:
#   container run aoa-agent:latest claude --dangerously-skip-permissions
ENTRYPOINT ["/usr/local/bin/aoa-entrypoint"]
CMD ["/bin/bash"]
