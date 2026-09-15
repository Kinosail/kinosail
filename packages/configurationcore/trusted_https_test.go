package configurationcore

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

type testSnapshot map[string]string

func (snapshot testSnapshot) String(key string) string { return snapshot[key] }
func (snapshot testSnapshot) Bool(key string) bool     { return snapshot[key] == "true" }

func trustedRaw(t *testing.T, provider, hostname string) string {
	t.Helper()
	config, err := trustedhttps.NewProviderConfig(provider, hostname, strings.Repeat("t", 32), "192.168.1.10", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := config.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTrustedHTTPSValidation(t *testing.T) {
	t.Parallel()
	duckDNS := trustedRaw(t, trustedhttps.ProviderDuckDNS, "family")
	base := testSnapshot{"tls.duckdns": duckDNS, "tls.enabled": "true", "remote.mode": "off", "listen": ":443"}
	if err := ValidateTrustedHTTPS(base); err != nil {
		t.Fatal(err)
	}
	base["auth.url"] = "https://family.duckdns.org"
	if err := ValidateTrustedHTTPS(base); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(testSnapshot){
		"invalid document": func(s testSnapshot) { s["tls.duckdns"] = "{" },
		"TLS disabled":     func(s testSnapshot) { s["tls.enabled"] = "false" },
		"public HTTPS":     func(s testSnapshot) { s["remote.mode"] = "https" },
		"same subdomain": func(s testSnapshot) {
			s["remote.mode"], s["remote.duckdns_domain"] = "proxy", "family"
		},
		"invalid listen": func(s testSnapshot) { s["listen"] = "invalid" },
		"wrong auth URL": func(s testSnapshot) { s["auth.url"] = "https://other.example" },
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := testSnapshot{"tls.duckdns": duckDNS, "tls.enabled": "true", "remote.mode": "off", "listen": ":443"}
			change(snapshot)
			if err := ValidateTrustedHTTPS(snapshot); err == nil {
				t.Fatal("invalid trusted HTTPS configuration was accepted")
			}
		})
	}
	if err := ValidateTrustedHTTPS(testSnapshot{}); err != nil {
		t.Fatalf("disabled trusted HTTPS = %v", err)
	}
	desec := testSnapshot{"tls.duckdns": trustedRaw(t, trustedhttps.ProviderDeSEC, "family.dedyn.io"), "tls.enabled": "true", "remote.mode": "proxy", "remote.duckdns_domain": "family.dedyn.io", "listen": ":443"}
	if err := ValidateTrustedHTTPS(desec); err != nil {
		t.Fatal(err)
	}
}

func TestTrustedOrigin(t *testing.T) {
	t.Parallel()
	raw := trustedRaw(t, trustedhttps.ProviderDuckDNS, "family")
	for listen, want := range map[string]string{
		":443":   "https://family.duckdns.org",
		":38127": "https://family.duckdns.org:38127",
	} {
		origin, err := TrustedOrigin(raw, listen)
		if err != nil || origin != want {
			t.Errorf("TrustedOrigin(%q) = %q, %v", listen, origin, err)
		}
	}
	for _, input := range []struct{ raw, listen string }{
		{"{", ":443"},
		{raw, "invalid"},
		{raw, ":not-a-port"},
		{raw, ":0"},
		{raw, ":65536"},
	} {
		if _, err := TrustedOrigin(input.raw, input.listen); err == nil {
			t.Errorf("TrustedOrigin(%q, %q) succeeded", input.raw, input.listen)
		}
	}
	if origin, err := TrustedOrigin("", ":443"); err != nil || origin != "" {
		t.Fatalf("disabled TrustedOrigin = %q, %v", origin, err)
	}
}
