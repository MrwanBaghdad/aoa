#!/bin/sh
# aoa-entrypoint — runs inside the container VM
# 1. Applies network policy (iptables-legacy)
# 2. Launches the requested agent
#
# Environment variables injected by aoa:
#   AOA_NETWORK_MODE   — restricted | allowlist | open
#   AOA_SESSION_ID     — session UUID

set -e

AOA_NETWORK_MODE="${AOA_NETWORK_MODE:-restricted}"
IPT=iptables-legacy

apply_network_policy() {
    case "$AOA_NETWORK_MODE" in
        open)
            echo "[aoa] Network: open (unrestricted)"
            ;;
        allowlist)
            echo "[aoa] Network: allowlist mode"
            $IPT -F OUTPUT 2>/dev/null || true
            $IPT -P OUTPUT DROP
            $IPT -A OUTPUT -o lo -j ACCEPT
            $IPT -A OUTPUT -p udp --dport 53 -j ACCEPT
            $IPT -A OUTPUT -p tcp --dport 53 -j ACCEPT
            $IPT -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
            # Allowlist entries are added by aoa via AOA_ALLOWLIST env var (comma-separated)
            if [ -n "$AOA_ALLOWLIST" ]; then
                echo "$AOA_ALLOWLIST" | tr ',' '\n' | while read -r entry; do
                    [ -n "$entry" ] && $IPT -A OUTPUT -d "$entry" -j ACCEPT
                done
            fi
            echo "[aoa] Allowlist applied"
            ;;
        *)
            # Default: restricted
            echo "[aoa] Network: restricted (private networks blocked)"
            $IPT -F OUTPUT 2>/dev/null || true
            $IPT -P OUTPUT DROP
            $IPT -A OUTPUT -o lo -j ACCEPT
            $IPT -A OUTPUT -p udp --dport 53 -j ACCEPT
            $IPT -A OUTPUT -p tcp --dport 53 -j ACCEPT
            $IPT -A OUTPUT -d 10.0.0.0/8 -j DROP
            $IPT -A OUTPUT -d 172.16.0.0/12 -j DROP
            $IPT -A OUTPUT -d 192.168.0.0/16 -j DROP
            $IPT -A OUTPUT -d 169.254.169.254/32 -j DROP
            $IPT -A OUTPUT -d 100.100.100.200/32 -j DROP
            $IPT -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
            $IPT -A OUTPUT -p tcp --dport 443 -j ACCEPT
            $IPT -A OUTPUT -p tcp --dport 80 -j ACCEPT
            ;;
    esac
}

# Apply network policy (requires root — apple/container runs as root by default)
if [ "$(id -u)" = "0" ]; then
    apply_network_policy
    echo "[aoa] OUTPUT policy: $($IPT -S OUTPUT 2>&1 | head -10)"

    # --allow-host: punch a hole to the host machine's gateway IP.
    # Inserted at position 1 so it takes priority over private-network DROP rules.
    if [ "${AOA_ALLOW_HOST:-0}" = "1" ]; then
        GATEWAY=$(ip route show default 2>/dev/null | awk '/default/ {print $3; exit}')
        if [ -n "$GATEWAY" ]; then
            if [ -n "${AOA_ALLOW_HOST_PORTS:-}" ]; then
                echo "$AOA_ALLOW_HOST_PORTS" | tr ',' '\n' | while read -r port; do
                    [ -n "$port" ] && $IPT -I OUTPUT 1 -d "$GATEWAY" -p tcp --dport "$port" -j ACCEPT
                done
                echo "[aoa] Host access: $GATEWAY ports $AOA_ALLOW_HOST_PORTS"
            else
                $IPT -I OUTPUT 1 -d "$GATEWAY" -j ACCEPT
                echo "[aoa] Host access: $GATEWAY (all ports)"
            fi
        else
            echo "[aoa] Warning: --allow-host set but could not detect gateway IP"
        fi
    fi
else
    echo "[aoa] Warning: not running as root, skipping network policy"
fi

echo "[aoa] Session: ${AOA_SESSION_ID:-unknown}"
echo "[aoa] Workspace: $(ls /workspace 2>/dev/null | head -5 | tr '\n' ' ')"

# Align the agent user's UID/GID with the workspace mount owner so that files
# created inside the VM appear correctly owned on the host filesystem.
# Skip if workspace is root-owned (UID 0) — that's an unusual host config.
WORKSPACE_UID=$(stat -c '%u' /workspace 2>/dev/null || echo 1000)
WORKSPACE_GID=$(stat -c '%g' /workspace 2>/dev/null || echo 1000)
if [ "$WORKSPACE_UID" != "0" ]; then
    usermod -u "$WORKSPACE_UID" agent 2>/dev/null || true
    groupmod -g "$WORKSPACE_GID" agent 2>/dev/null || true
fi
echo "[aoa] Agent: uid=$WORKSPACE_UID gid=$WORKSPACE_GID"

# Drop CAP_NET_ADMIN and CAP_NET_RAW from the bounding set and switch to the
# non-root agent user in one step. capsh applies capability changes first,
# then switches uid/gid, then execs — so the agent process is non-root with
# immutable network rules. Claude Code requires non-root to accept
# --dangerously-skip-permissions.
cd /workspace
exec capsh \
    --drop=cap_net_admin,cap_net_raw \
    --user=agent \
    -- -c 'exec "$0" "$@"' "$@"
