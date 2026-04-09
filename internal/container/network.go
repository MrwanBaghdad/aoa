package container

import (
	"fmt"
	"strings"
)

// NetworkMode defines the egress filtering policy.
type NetworkMode string

const (
	NetworkModeRestricted NetworkMode = "restricted"
	NetworkModeAllowlist  NetworkMode = "allowlist"
	NetworkModeOpen       NetworkMode = "open"
)

// NetworkPolicy generates the iptables-legacy rules to be applied inside the VM.
// These rules are written to a script that the container entrypoint executes.
type NetworkPolicy struct {
	Mode      NetworkMode
	Allowlist []string // for allowlist mode: allowed IPs/CIDRs
}

// Script returns a shell script fragment that applies the iptables rules.
// Must be run as root inside the container (before agent starts).
// Uses iptables-legacy — required for apple/container VMs (nftables not supported).
func (p NetworkPolicy) Script() string {
	var b strings.Builder

	b.WriteString("#!/bin/sh\n")
	b.WriteString("# Network policy: " + string(p.Mode) + "\n")
	b.WriteString("IPT=iptables-legacy\n\n")

	switch p.Mode {
	case NetworkModeOpen:
		b.WriteString("# Open mode — no restrictions\n")
		b.WriteString("echo 'Network: open (unrestricted)'\n")

	case NetworkModeRestricted:
		b.WriteString(restrictedRules())

	case NetworkModeAllowlist:
		b.WriteString(allowlistRules(p.Allowlist))
	}

	return b.String()
}

func restrictedRules() string {
	return `# Restricted mode — allow internet HTTPS/DNS, block private networks and metadata
$IPT -F OUTPUT 2>/dev/null || true
$IPT -P OUTPUT DROP

# Allow loopback
$IPT -A OUTPUT -o lo -j ACCEPT

# Allow DNS (UDP + TCP)
$IPT -A OUTPUT -p udp --dport 53 -j ACCEPT
$IPT -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Block private networks (lateral movement prevention)
$IPT -A OUTPUT -d 10.0.0.0/8 -j DROP
$IPT -A OUTPUT -d 172.16.0.0/12 -j DROP
$IPT -A OUTPUT -d 192.168.0.0/16 -j DROP

# Block cloud metadata endpoints
$IPT -A OUTPUT -d 169.254.169.254/32 -j DROP
$IPT -A OUTPUT -d 100.100.100.200/32 -j DROP

# Allow established connections
$IPT -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Allow HTTPS (LLM API calls, package installs)
$IPT -A OUTPUT -p tcp --dport 443 -j ACCEPT

# Allow HTTP (package registries — some don't support HTTPS)
$IPT -A OUTPUT -p tcp --dport 80 -j ACCEPT

echo 'Network: restricted (private networks blocked, internet allowed)'
`
}

func allowlistRules(allowlist []string) string {
	var b strings.Builder
	b.WriteString(`# Allowlist mode — deny all, allow only specified IPs/CIDRs
$IPT -F OUTPUT 2>/dev/null || true
$IPT -P OUTPUT DROP

# Allow loopback
$IPT -A OUTPUT -o lo -j ACCEPT

# Allow DNS
$IPT -A OUTPUT -p udp --dport 53 -j ACCEPT
$IPT -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Allow established connections
$IPT -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

`)
	for _, entry := range allowlist {
		fmt.Fprintf(&b, "$IPT -A OUTPUT -d %s -j ACCEPT\n", entry)
	}
	b.WriteString("\necho 'Network: allowlist'\n")
	return b.String()
}
