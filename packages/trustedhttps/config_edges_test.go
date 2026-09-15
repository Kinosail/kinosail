package trustedhttps

import (
	"strings"
	"testing"
)

func TestConfigurationCoversCanonicalDefaultsAndFailures(t *testing.T) {
	config, err := NewProviderConfig("", " Family.duckdns.org. ", " "+testToken+" ", " 192.168.1.10 ", true)
	if err != nil || config.Provider != ProviderDuckDNS || config.Domain != "family" {
		t.Fatalf("default provider config = %#v, %v", config, err)
	}
	legacy := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	encoded, err := legacy.Encode()
	if err != nil || !strings.Contains(encoded, `"provider":"duckdns"`) {
		t.Fatalf("legacy encoding = %q, %v", encoded, err)
	}
	if _, err = (Config{Domain: "family"}).Encode(); err == nil {
		t.Fatal("invalid configuration encoded")
	}
	if (Config{}).ProviderName() != "" || legacy.ProviderName() != ProviderDuckDNS {
		t.Fatal("provider defaults are not canonical")
	}
}

func TestParseRejectsEveryDocumentBoundary(t *testing.T) {
	validInvalidConfig := `{"provider":"duckdns","domain":"family","token":"short","address":"192.168.1.10","termsAccepted":true}`
	for name, raw := range map[string]string{
		"too large":        strings.Repeat("x", 2049),
		"wrong field type": `{"domain":1}`,
		"missing close":    `{"domain":"family"`,
		"wrong close":      `{]`,
		"invalid config":   validInvalidConfig,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatalf("invalid document %q accepted", raw)
			}
		})
	}
}
