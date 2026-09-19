package validation

import (
	"net/url"
	"strings"
	"testing"
)

func TestValidateRemoteAccess(t *testing.T) {
	t.Parallel()
	valid := map[string]string{"remote.mode": "https", "remote.listen": ":8443", "remote.duckdns_domain": "family", "remote.duckdns_token": strings.Repeat("a", 32), "auth.url": "https://family.duckdns.org"}
	value := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	if err := ValidateRemoteAccess(value(valid)); err != nil {
		t.Fatalf("ValidateRemoteAccess(valid) = %v", err)
	}
	if err := ValidateRemoteAccess(nil); err == nil {
		t.Fatal("ValidateRemoteAccess(nil) succeeded")
	}
	for name, mutate := range map[string]func(map[string]string){
		"listen":        func(values map[string]string) { values["remote.listen"] = "bad" },
		"credentials":   func(values map[string]string) { values["remote.duckdns_token"] = "" },
		"unknown mode":  func(values map[string]string) { values["remote.mode"] = "invalid" },
		"origin":        func(values map[string]string) { values["auth.url"] = "https://other.example" },
		"origin scheme": func(values map[string]string) { values["auth.url"] = "http://family.duckdns.org" },
	} {
		t.Run(name, func(t *testing.T) {
			values := clone(valid)
			mutate(values)
			if err := ValidateRemoteAccess(value(values)); err == nil {
				t.Fatal("ValidateRemoteAccess() succeeded")
			}
		})
	}
	off := clone(valid)
	off["remote.mode"] = "off"
	if err := ValidateRemoteAccess(value(off)); err != nil {
		t.Fatalf("ValidateRemoteAccess(off) = %v", err)
	}
}

func TestRemoteTokenValidation(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"", strings.Repeat("a", 31), strings.Repeat("a", 129), strings.Repeat("a", 31) + "!", strings.Repeat("a", 31) + "é"} {
		if ValidRemoteToken(token) {
			t.Fatalf("unsafe remote token accepted: %q", token)
		}
	}
	if !ValidRemoteToken(strings.Repeat("aA0-_", 7)) {
		t.Fatal("supported remote token rejected")
	}
	for _, token := range []string{strings.Repeat("a", 32), strings.Repeat("a", 128), strings.Repeat("a", 31) + "Z", strings.Repeat("a", 31) + "_"} {
		if !ValidRemoteToken(token) {
			t.Fatalf("boundary remote token rejected: %q", token)
		}
	}
}

func TestSCIMTokenValidation(t *testing.T) {
	t.Parallel()
	for token, want := range map[string]bool{strings.Repeat("x", 32): true, strings.Repeat("x", 31): false, strings.Repeat("x", 257): false, strings.Repeat("x", 31) + " ": false} {
		if got := ValidSCIMToken(token); got != want {
			t.Errorf("ValidSCIMToken() = %t, want %t", got, want)
		}
	}
	for _, token := range []string{strings.Repeat("x", 256), strings.Repeat("x", 31) + "!", strings.Repeat("x", 31) + "~"} {
		if !ValidSCIMToken(token) {
			t.Fatalf("boundary SCIM token rejected: %q", token)
		}
	}
}

func TestSupporterValidation(t *testing.T) {
	t.Parallel()
	if err := ValidateSupporter("https://support.example/activate", "https://support.example"); err != nil {
		t.Fatalf("ValidateSupporter(valid) = %v", err)
	}
	if err := ValidateSupporter("http://localhost:8080/activate", "https://support.example"); err != nil {
		t.Fatalf("ValidateSupporter(loopback) = %v", err)
	}
	for name, endpoints := range map[string][2]string{
		"activation query":       {"https://support.example/activate?secret=x", "https://support.example"},
		"activation credentials": {"https://user@support.example/activate", "https://support.example"},
		"support HTTP":           {"https://support.example/activate", "http://support.example"},
		"support fragment":       {"https://support.example/activate", "https://support.example/#fragment"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSupporter(endpoints[0], endpoints[1]); err == nil {
				t.Fatal("ValidateSupporter() succeeded")
			}
		})
	}
	if ValidSupporterURL(nil, true) {
		t.Fatal("ValidSupporterURL(nil) succeeded")
	}
	parsed, _ := url.Parse("http://127.0.0.1:8080")
	if !ValidSupporterURL(parsed, true) || ValidSupporterURL(parsed, false) {
		t.Fatal("loopback HTTP policy mismatch")
	}
	private, _ := url.Parse("http://192.168.1.10:8080")
	if ValidSupporterURL(private, true) {
		t.Fatal("non-loopback HTTP endpoint accepted")
	}
}

func TestAllRunes(t *testing.T) {
	t.Parallel()
	if !allRunes("a", func(rune) bool { return true }) {
		t.Fatal("allRunes rejected an allowed character")
	}
	if allRunes("a", func(rune) bool { return false }) {
		t.Fatal("allRunes accepted a rejected character")
	}
}

func clone(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func TestPublicGatewayRequiresExplicitHTTPSMode(t *testing.T) {
	for _, input := range []struct{ mode, gateway string }{
		{"off", "true"}, {"https", "TRUE"}, {"https", "1"}, {"unknown", "false"},
	} {
		values := map[string]string{"remote.mode": input.mode, "remote.gateway": input.gateway, "remote.listen": ":8443", "remote.duckdns_domain": "family", "remote.duckdns_token": "token", "auth.url": "https://family.duckdns.org"}
		if ValidateRemoteAccess(func(key string) string { return values[key] }) == nil {
			t.Fatalf("accepted %v", input)
		}
	}
}
