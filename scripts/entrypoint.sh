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

# Hand off to the agent or shell
cd /workspace
exec "$@"
