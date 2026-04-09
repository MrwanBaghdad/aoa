package container

import (
	"strings"
	"testing"
)

func TestRestrictedModeScript(t *testing.T) {
	p := NetworkPolicy{Mode: NetworkModeRestricted}
	script := p.Script()

	mustContain := []string{
		"iptables-legacy",
		"OUTPUT DROP",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.169.254",
		"--dport 53",
		"--dport 443",
		"-o lo -j ACCEPT",
	}
	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("restricted script missing %q", s)
		}
	}
}

func TestOpenModeScript(t *testing.T) {
	p := NetworkPolicy{Mode: NetworkModeOpen}
	script := p.Script()

	if !strings.Contains(script, "open") && !strings.Contains(script, "unrestricted") {
		t.Error("open mode script should mention 'open' or 'unrestricted'")
	}
	// Open mode must NOT add DROP rules
	if strings.Contains(script, "OUTPUT DROP") {
		t.Error("open mode script must not add OUTPUT DROP rules")
	}
}

func TestAllowlistModeScript(t *testing.T) {
	p := NetworkPolicy{
		Mode:      NetworkModeAllowlist,
		Allowlist: []string{"203.0.113.5", "8.8.8.8/32"},
	}
	script := p.Script()

	if !strings.Contains(script, "OUTPUT DROP") {
		t.Error("allowlist mode must default-deny output")
	}
	if !strings.Contains(script, "203.0.113.5") {
		t.Error("allowlist script missing first allowed IP")
	}
	if !strings.Contains(script, "8.8.8.8/32") {
		t.Error("allowlist script missing second allowed IP")
	}
	if !strings.Contains(script, "--dport 53") {
		t.Error("allowlist script must allow DNS")
	}
}

func TestAllowlistModeEmptyAllowlist(t *testing.T) {
	p := NetworkPolicy{Mode: NetworkModeAllowlist, Allowlist: nil}
	script := p.Script()
	// Should still produce a valid deny-all script
	if !strings.Contains(script, "OUTPUT DROP") {
		t.Error("empty allowlist should still default-deny output")
	}
}

func TestScriptUsesIptablesLegacyVariable(t *testing.T) {
	for _, mode := range []NetworkMode{NetworkModeRestricted, NetworkModeAllowlist} {
		p := NetworkPolicy{Mode: mode}
		script := p.Script()
		if !strings.Contains(script, "IPT=iptables-legacy") {
			t.Errorf("mode %s: script must set IPT=iptables-legacy", mode)
		}
	}
}

func TestScriptHasShebang(t *testing.T) {
	p := NetworkPolicy{Mode: NetworkModeRestricted}
	script := p.Script()
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Error("script must start with #!/bin/sh")
	}
}

func TestNetworkModeConstants(t *testing.T) {
	if NetworkModeRestricted != "restricted" {
		t.Errorf("unexpected value: %q", NetworkModeRestricted)
	}
	if NetworkModeAllowlist != "allowlist" {
		t.Errorf("unexpected value: %q", NetworkModeAllowlist)
	}
	if NetworkModeOpen != "open" {
		t.Errorf("unexpected value: %q", NetworkModeOpen)
	}
}
