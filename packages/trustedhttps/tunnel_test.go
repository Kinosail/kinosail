package trustedhttps

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestTunnelCertificateRenewsWithoutPublishingPrivateAddress(t *testing.T) {
	manager, err := NewTunnelCertificate("family", testToken, "10.92.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// This provider fails any address update; certificate-only maintenance must not call it.
	manager.provider = failingDNSProvider{}
	obtained := 0
	manager.obtain = func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
		obtained++
		return testIdentity(t, "family.duckdns.org", time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour)), nil
	}
	if err = manager.maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if obtained != 1 || manager.Certificate("family.duckdns.org") == nil {
		t.Fatal("private HTTPS did not renew")
	}
	if err = manager.maintain(t.Context()); err != nil || obtained != 1 {
		t.Fatal("valid certificate was renewed unnecessarily")
	}
}

func TestTunnelCertificateRejectsInvalidConfigurationBeforeFiles(t *testing.T) {
	for _, input := range []struct{ domain, token, address string }{
		{"", testToken, "10.92.0.1"}, {"BAD DOMAIN", testToken, "10.92.0.1"},
		{"family", "", "10.92.0.1"}, {"family", "short", "10.92.0.1"},
		{"family", testToken, "8.8.8.8"}, {"family", testToken, "not-an-address"},
	} {
		dir := t.TempDir()
		if _, err := NewTunnelCertificate(input.domain, input.token, input.address, dir); err == nil {
			t.Fatal("invalid private HTTPS accepted")
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatal("invalid configuration wrote files")
		}
	}
}
