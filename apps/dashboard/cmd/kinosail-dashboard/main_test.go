package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestRequestedCommandRejectsSecondStateOwner(t *testing.T) {
	if command, err := requestedCommand(nil); err != nil || command != "serve" {
		t.Fatalf("default command = %q, %v", command, err)
	}
	if _, err := requestedCommand([]string{"mcp-stdio"}); err == nil {
		t.Fatal("mcp-stdio must not open a second state owner")
	}
	if _, err := requestedCommand([]string{"serve", "extra"}); err == nil {
		t.Fatal("multiple commands must be rejected")
	}
}

func TestPublicProbeHostsRequireExactDNSNames(t *testing.T) {
	hosts, err := parsePublicHosts("Status.Example., media.example")
	if err != nil || !reflect.DeepEqual(hosts, []string{"status.example", "media.example"}) {
		t.Fatalf("hosts = %#v, %v", hosts, err)
	}
	for _, value := range []string{"203.0.113.10", "*.example", "bad_name.example", "one.example:443"} {
		if _, err := parsePublicHosts(value); err == nil {
			t.Fatalf("public probe host %q should be rejected", value)
		}
	}
}

func TestTrustedHostsAcceptDNSAndIPButRejectAmbiguity(t *testing.T) {
	hosts, err := parseTrustedHosts("dashboard.example, [2001:db8::1]")
	if err != nil || !reflect.DeepEqual(hosts, []string{"dashboard.example", "2001:db8::1"}) {
		t.Fatalf("hosts = %#v, %v", hosts, err)
	}
	for _, value := range []string{"*.example", "bad_name.example", "example.test:443"} {
		if _, err := parseTrustedHosts(value); err == nil {
			t.Fatalf("trusted host %q should be rejected", value)
		}
	}
}

func TestPublicURLIsOneBoundedOrigin(t *testing.T) {
	if got, err := parsePublicURL("https://Dashboard.Example:38400/", ":38400"); err != nil || got != "https://Dashboard.Example:38400" {
		t.Fatalf("public URL = %q, %v", got, err)
	}
	if got, err := parsePublicURL("", ":38400"); err != nil || got != "http://localhost:38400" {
		t.Fatalf("default public URL = %q, %v", got, err)
	}
	if got, err := parsePublicURL("", "127.0.0.1:38400"); err != nil || got != "http://localhost:38400" {
		t.Fatalf("loopback public URL = %q, %v", got, err)
	}
	for _, value := range []string{"ftp://dashboard.example", "https://user@example.test", "https://example.test/path", "https://example.test?query=1", "https://example.test#fragment", "https://" + strings.Repeat("a", 2049)} {
		if _, err := parsePublicURL(value, ":38400"); err == nil {
			t.Fatalf("public URL %q should be rejected", value)
		}
	}
}

func TestRuntimeConfigRequiresSecureCookiesForHTTPSPublicURL(t *testing.T) {
	t.Setenv("KINOSAIL_DASHBOARD_LISTEN", "127.0.0.1:38400")
	t.Setenv("KINOSAIL_DASHBOARD_DATA_DIR", t.TempDir())
	t.Setenv("KINOSAIL_DASHBOARD_PUBLIC_URL", "https://dashboard.example:38400")
	t.Setenv("KINOSAIL_DASHBOARD_SECURE_COOKIES", "false")
	t.Setenv("KINOSAIL_DASHBOARD_PROBE_INTERVAL", "")
	t.Setenv("KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS", "")
	t.Setenv("KINOSAIL_DASHBOARD_TRUSTED_HOSTS", "dashboard.example")
	if _, err := loadRuntimeConfig(); err == nil {
		t.Fatal("HTTPS public URL accepted insecure cookies")
	}
	t.Setenv("KINOSAIL_DASHBOARD_PUBLIC_URL", "http://dashboard.example:38400")
	t.Setenv("KINOSAIL_DASHBOARD_SECURE_COOKIES", "true")
	if _, err := loadRuntimeConfig(); err == nil {
		t.Fatal("HTTP public URL accepted secure cookies")
	}
	t.Setenv("KINOSAIL_DASHBOARD_PUBLIC_URL", "https://dashboard.example:38400")
	t.Setenv("KINOSAIL_DASHBOARD_SECURE_COOKIES", "true")
	configured, err := loadRuntimeConfig()
	if err != nil || configured.publicURL != "https://dashboard.example:38400" {
		t.Fatalf("runtime config = %#v, %v", configured, err)
	}
}
