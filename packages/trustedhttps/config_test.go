package trustedhttps

import (
	"context"
	"net"
	"strings"
	"testing"
)

var testToken = strings.Repeat("t", 32)

func TestConfigRoundTrip(t *testing.T) {
	t.Parallel()

	config, err := NewProviderConfig(ProviderDuckDNS, " Family ", testToken, " 192.168.1.10 ", true)
	if err != nil {
		t.Fatal(err)
	}
	if config.Domain != "family" || config.Hostname() != "family.duckdns.org" || config.Address != "192.168.1.10" {
		t.Fatalf("unexpected normalized config: %#v", config)
	}
	raw, err := config.Encode()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != config {
		t.Fatalf("round trip = %#v, want %#v", parsed, config)
	}
	if raw == "" || !strings.Contains(raw, testToken) {
		t.Fatalf("encoded secret document is incomplete: %q", raw)
	}
}

func TestConfigAcceptsLocalHostnameAddress(t *testing.T) {
	t.Parallel()

	config, err := NewProviderConfig(ProviderDuckDNS, "family", testToken, "server.nox", true)
	if err != nil {
		t.Fatalf("local hostname address rejected: %v", err)
	}
	if config.Address != "server.nox" {
		t.Fatalf("address = %q, want server.nox", config.Address)
	}
}

func TestLocalHostnameResolutionRequiresOnePrivateIPv4(t *testing.T) {
	t.Parallel()

	for name, addresses := range map[string][]net.IP{
		"public":   {net.ParseIP("203.0.113.2")},
		"multiple": {net.ParseIP("192.168.1.10"), net.ParseIP("192.168.1.11")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := (Config{Domain: "family", Token: testToken, Address: "server.nox", Terms: true}).withResolvedAddress(t.Context(), func(context.Context, string) ([]net.IP, error) {
				return addresses, nil
			})
			if err == nil {
				t.Fatal("unsafe hostname resolution succeeded")
			}
		})
	}
}

func TestProviderConfigsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		provider, input, hostname string
	}{
		{ProviderDeSEC, " Family.dedyn.io ", "family.dedyn.io"},
		{ProviderDuckDNS, " Family.duckdns.org ", "family.duckdns.org"},
	} {
		t.Run(test.provider, func(t *testing.T) {
			config, err := NewProviderConfig(test.provider, test.input, testToken, "192.168.1.10", true)
			if err != nil {
				t.Fatal(err)
			}
			if config.ProviderName() != test.provider || config.Hostname() != test.hostname {
				t.Fatalf("config = %#v, hostname = %q", config, config.Hostname())
			}
			raw, err := config.Encode()
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(raw)
			if err != nil || parsed != config {
				t.Fatalf("round trip = %#v, %v", parsed, err)
			}
		})
	}
}

func TestLegacyDuckDNSDocumentMigratesWithoutOwnerAction(t *testing.T) {
	t.Parallel()
	raw := `{"domain":"family","token":"` + testToken + `","address":"192.168.1.10","termsAccepted":true}`
	config, err := Parse(raw)
	if err != nil || config.ProviderName() != ProviderDuckDNS || config.Hostname() != "family.duckdns.org" {
		t.Fatalf("legacy config = %#v, %v", config, err)
	}
}

func TestParseDisabled(t *testing.T) {
	t.Parallel()

	config, err := Parse("")
	if err != nil || config != (Config{}) {
		t.Fatalf("Parse empty = %#v, %v", config, err)
	}
}

func TestConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	tests := map[string]Config{
		"missing domain":      {Token: testToken, Address: valid.Address, Terms: true},
		"full hostname":       {Domain: "family.duckdns.org", Token: testToken, Address: valid.Address, Terms: true},
		"uppercase stored":    {Domain: "Family", Token: testToken, Address: valid.Address, Terms: true},
		"leading hyphen":      {Domain: "-family", Token: testToken, Address: valid.Address, Terms: true},
		"short token":         {Domain: valid.Domain, Token: "short", Address: valid.Address, Terms: true},
		"unsafe token":        {Domain: valid.Domain, Token: strings.Repeat("a", 31) + "/", Address: valid.Address, Terms: true},
		"public address":      {Domain: valid.Domain, Token: testToken, Address: "203.0.113.2", Terms: true},
		"loopback address":    {Domain: valid.Domain, Token: testToken, Address: "127.0.0.1", Terms: true},
		"IPv6 address":        {Domain: valid.Domain, Token: testToken, Address: "fd00::1", Terms: true},
		"unaccepted terms":    {Domain: valid.Domain, Token: testToken, Address: valid.Address},
		"missing token":       {Domain: valid.Domain, Address: valid.Address, Terms: true},
		"missing LAN address": {Domain: valid.Domain, Token: testToken, Terms: true},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := config.Validate(); err == nil {
				t.Fatalf("Validate(%#v) succeeded", config)
			}
		})
	}
}

func TestProviderConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]struct{ provider, hostname, token string }{
		"unsupported provider": {"dynv6", "family.example", testToken},
		"single label":         {ProviderDeSEC, "family", testToken},
		"wrong DuckDNS":        {ProviderDuckDNS, "family.example", testToken},
		"unsafe hostname":      {ProviderDeSEC, "family..dedyn.io", testToken},
		"header injection":     {ProviderDeSEC, "family.dedyn.io", testToken + "\nattack"},
		"oversized token":      {ProviderDeSEC, "family.dedyn.io", strings.Repeat("t", 513)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewProviderConfig(input.provider, input.hostname, input.token, "192.168.1.10", true); err == nil {
				t.Fatal("invalid provider configuration was accepted")
			}
		})
	}
}

func TestParseRejectsNonCanonicalDocuments(t *testing.T) {
	t.Parallel()

	valid := `{"domain":"family","token":"` + testToken + `","address":"192.168.1.10","termsAccepted":true}`
	tests := map[string]string{
		"malformed":        `{`,
		"unknown field":    strings.TrimSuffix(valid, "}") + `,"extra":true}`,
		"second object":    valid + `{}`,
		"trailing garbage": valid + ` garbage`,
		"null":             `null`,
		"duplicate field":  strings.Replace(valid, `"domain":"family"`, `"domain":"family","domain":"other"`, 1),
		"oversized":        strings.Repeat("x", 1025),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(raw); err == nil {
				t.Fatalf("Parse(%q) succeeded", raw)
			}
		})
	}
}
