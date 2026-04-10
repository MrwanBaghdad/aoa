#!/bin/bash
# aoa-test-security — run INSIDE the container to verify security properties
# Usage: container exec <id> aoa-test-security
#        or: aoa shell --agent bash, then run aoa-test-security

PASS=0
FAIL=0

test_case() {
    local name="$1"
    local cmd="$2"
    local expect_fail="$3"  # "true" means the command should fail/be blocked

    if eval "$cmd" > /dev/null 2>&1; then
        if [ "$expect_fail" = "true" ]; then
            echo "FAIL  $name (should have been blocked)"
            FAIL=$((FAIL+1))
        else
            echo "PASS  $name"
            PASS=$((PASS+1))
        fi
    else
        if [ "$expect_fail" = "true" ]; then
            echo "PASS  $name (correctly blocked)"
            PASS=$((PASS+1))
        else
            echo "FAIL  $name (should have worked)"
            FAIL=$((FAIL+1))
        fi
    fi
}

echo "=== aoa security test suite ==="
echo ""

echo "--- Credential Isolation ---"
test_case "No host SSH keys"    "test -f ~/.ssh/id_rsa || test -f ~/.ssh/id_ed25519"       "true"
test_case "No host .env files"  "find / -name '.env' 2>/dev/null | grep -v /workspace | grep -q ." "true"
test_case "No password in env"  "env | grep -qi 'password'" "true"
test_case "No token in env"     "env | grep -qi 'github_token\|gh_token\|npm_token'" "true"
test_case "API key available"   "test -n \"\$ANTHROPIC_API_KEY\""  "false"

echo ""
echo "--- VM Isolation ---"
test_case "No host filesystem"    "test -d /Users"           "true"
test_case "No host /home outside" "test -d /home/marwan"     "true"
test_case "Own kernel"            "test -f /proc/version"    "false"
test_case "Workspace mounted"     "test -d /workspace"       "false"

echo ""
echo "--- Network Isolation (restricted mode) ---"
test_case "Can reach HTTPS"      "curl -s --max-time 5 -o /dev/null https://api.anthropic.com/health" "false"
test_case "Blocks private 192.x" "curl -s --max-time 3 http://192.168.1.1"                            "true"
test_case "Blocks private 10.x"  "curl -s --max-time 3 http://10.0.0.1"                              "true"
test_case "Blocks metadata"      "curl -s --max-time 3 http://169.254.169.254"                        "true"
test_case "DNS works"            "nslookup api.anthropic.com"                                          "false"

echo ""
echo "--- Supply Chain Protection ---"
test_case ".git/hooks read-only" "test -d /workspace/.git/hooks && touch /workspace/.git/hooks/test-write 2>/dev/null" "true"

echo ""
echo "--- Network Policy Immutability ---"
test_case "Cannot flush iptables (CAP_NET_ADMIN dropped)" \
    "iptables-legacy -F OUTPUT 2>/dev/null" "true"
test_case "Cannot modify iptables rules" \
    "iptables-legacy -A OUTPUT -j ACCEPT 2>/dev/null" "true"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
