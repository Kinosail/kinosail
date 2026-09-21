package serverdiscovery

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDiscoveryPublicBoundaryRejectsWithoutAdvertising(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Start(ctx, "Player", "https://localhost:8127"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled advertisement=%v", err)
	}
	if err := Start(t.Context(), "", "invalid"); err == nil {
		t.Fatal("invalid advertisement accepted")
	}
	if got := LocalAliases("invalid"); len(got) != 0 {
		t.Fatalf("invalid aliases=%v", got)
	}
	if validHostname(strings.Repeat("a", 254)) {
		t.Fatal("oversized hostname accepted")
	}
}

func TestLocalAliasesReflectOnlyValidatedMachineName(t *testing.T) {
	t.Parallel()
	alias := LoopbackAlias("https://127.0.0.1:8127")
	aliases := LocalAliases("https://127.0.0.1:8127")
	if alias == "" {
		if len(aliases) != 0 {
			t.Fatal("invalid machine name produced aliases")
		}
		return
	}
	if !validHostname(alias) || len(aliases) == 0 || aliases[0] != alias+":8127" {
		t.Fatalf("alias=%s addresses=%v", alias, aliases)
	}
	hosts := aliasHosts("fd00::1", "https", 443)
	if len(hosts) != 2 || hosts[1] != "[fd00::1]" {
		t.Fatalf("IPv6 aliases=%v", hosts)
	}
}
