package trustedhttps

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestManagerDefaultsAndErrorStateAreFailClosed(t *testing.T) { //nolint:cyclop // Constructor defaults and the renewal error state form one manager boundary.
	t.Parallel()
	if manager, err := New(Config{}, ""); err != nil || manager.Status().State != "disabled" || manager.Certificate("anything") != nil {
		t.Fatalf("disabled manager = %#v, %v", manager, err)
	}
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	if _, err := New(config, ""); err == nil {
		t.Fatal("missing data directory accepted")
	}
	for _, updateURL := range []string{"http://duck.test", "https:///missing", "https://user@duck.test", "https://duck.test?q=1", "https://duck.test/#fragment"} {
		if _, err := New(config, t.TempDir(), Dependencies{UpdateURL: updateURL}); err == nil {
			t.Fatalf("invalid update URL %q accepted", updateURL)
		}
	}
	manager, err := New(config, t.TempDir(), Dependencies{Client: &http.Client{Transport: failingTransport{}}, UpdateURL: "https://duck.test", Wait: func(context.Context, time.Duration) bool { return false }})
	if err != nil {
		t.Fatal(err)
	}
	manager.Run(t.Context())
	if status := manager.Status(); status.State != "error" || status.Error == "" {
		t.Fatalf("failed renewal status = %#v", status)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if waitForTXT(cancelled, "invalid.", "missing") == nil {
		t.Fatal("cancelled TXT wait succeeded")
	}
	blocked := t.TempDir() + "/blocked"
	if err = writePrivate(blocked, []byte("file")); err != nil {
		t.Fatal(err)
	}
	provider := duckDNSProvider{config: config, client: http.DefaultClient, endpoint: duckDNSUpdateURL}
	if _, err = obtainCertificate(t.Context(), config, blocked, http.DefaultClient, "", provider); err == nil {
		t.Fatal("blocked account-key directory accepted")
	}
}

func TestPublicOperationsRejectMultipleDependencySetsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	if _, err := New(config, directory, Dependencies{}, Dependencies{}); err == nil {
		t.Fatal("New accepted multiple dependency sets")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected New created %d file entries", len(entries))
	}
	transport := &countingTransport{}
	dependencies := Dependencies{Client: &http.Client{Transport: transport}}
	if err = Check(t.Context(), config, dependencies, dependencies); err == nil {
		t.Fatal("Check accepted multiple dependency sets")
	}
	if transport.requests != 0 {
		t.Fatalf("rejected Check made %d requests", transport.requests)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
}

type countingTransport struct{ requests int }

func (transport *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests++
	return nil, errors.New("unexpected request")
}
