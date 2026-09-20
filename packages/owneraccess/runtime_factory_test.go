package owneraccess

import (
	"context"
	"crypto/tls"
	"testing"
)

func TestRuntimeCertificateSourcesPreservePriorityAndFailClosed(t *testing.T) {
	config, _, _ := ownerFixture(t)
	trusted, internet := &tls.Certificate{}, &tls.Certificate{}
	calls := []string{}
	runtimeConfig := RuntimeConfig{
		Directory: config.Directory, Origin: config.Origin, Profile: config.Profile,
		TrustedCertificate:  func(name string) *tls.Certificate { calls = append(calls, "trusted:"+name); return trusted },
		InternetCertificate: func(name string) *tls.Certificate { calls = append(calls, "internet:"+name); return internet },
	}
	manager := OpenRuntime(runtimeConfig)
	if manager == nil {
		t.Fatal("valid management runtime unavailable")
	}
	hello := &tls.ClientHelloInfo{ServerName: "server.example"}
	certificate, err := manager.config.Certificate(hello)
	if err != nil || certificate != trusted || len(calls) != 1 {
		t.Fatal("trusted certificate priority lost")
	}
	trusted = nil
	certificate, err = manager.config.Certificate(hello)
	if err != nil || certificate != internet || len(calls) != 3 {
		t.Fatal("internet certificate fallback lost")
	}
	internet = nil
	assertRuntimeCertificatesFailClosed(t, manager, runtimeConfig, hello)
}

func assertRuntimeCertificatesFailClosed(t *testing.T, manager *Manager, runtimeConfig RuntimeConfig, hello *tls.ClientHelloInfo) {
	t.Helper()
	if certificate, err := manager.config.Certificate(hello); err == nil || certificate != nil {
		t.Fatal("missing trusted certificate accepted")
	}
	if err := manager.config.MaintainEndpoint(t.Context(), "other.example:51821"); err != nil {
		t.Fatal(err)
	}
	runtimeConfig.TrustedCertificate, runtimeConfig.InternetCertificate = nil, nil
	manager = OpenRuntime(runtimeConfig)
	if certificate, err := manager.config.Certificate(hello); err == nil || certificate != nil {
		t.Fatal("nil certificate sources accepted")
	}
}

func TestRuntimeRejectsMissingAndInvalidConfiguration(t *testing.T) {
	if OpenRuntime(RuntimeConfig{}) != nil {
		t.Fatal("empty directory enabled runtime")
	}
	config, _, _ := ownerFixture(t)
	manager := OpenRuntime(RuntimeConfig{Directory: config.Directory, Origin: "http://unsafe.example", Profile: config.Profile})
	if manager == nil || manager.Enable("home.example:51821") == nil || manager.Status().Enabled {
		t.Fatal("insecure origin enabled access")
	}
	if OpenRuntime(RuntimeConfig{Directory: "relative", Origin: config.Origin, Profile: config.Profile}) != nil {
		t.Fatal("invalid state directory accepted")
	}
	for _, origin := range []string{"%", "https://different.example"} {
		tunnel, _, _ := tunnelCertificate(RuntimeConfig{Directory: config.Directory, Origin: origin, DuckDNSDomain: "family", DuckDNSToken: "test-token"})
		if tunnel != nil {
			t.Fatal("invalid tunnel origin enabled certificate renewal")
		}
	}
}

func TestRuntimeTunnelUsesCanceledLifecycleWithoutDNSChanges(t *testing.T) {
	config, _, _ := ownerFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	manager := OpenRuntime(RuntimeConfig{Directory: config.Directory, Origin: "https://family.duckdns.org", Profile: config.Profile, DuckDNSDomain: "family", DuckDNSToken: "test-token", Lifecycle: ctx})
	if manager == nil {
		t.Fatal("tunnel runtime unavailable")
	}
	if err := manager.config.MaintainEndpoint(ctx, "family.duckdns.org:51821"); err == nil {
		t.Fatal("canceled DNS update reported success")
	}
	if certificate, err := manager.config.Certificate(&tls.ClientHelloInfo{ServerName: "family.duckdns.org"}); err == nil || certificate != nil {
		t.Fatal("unissued tunnel certificate accepted")
	}
}
