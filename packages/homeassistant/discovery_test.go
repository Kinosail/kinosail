package homeassistant

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

type testDiscovery struct{ stopped chan struct{} }

func (discovery *testDiscovery) Shutdown() {
	select {
	case <-discovery.stopped:
	default:
		close(discovery.stopped)
	}
}

func discoveryIntegration(t *testing.T, rawURL string, trusted TrustedHTTPS, advertise advertiseFunc) (*Integration[testProfile], context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	config := Config[testProfile]{
		Lifecycle:      ctx,
		AuthURL:        rawURL,
		Random:         bytes.NewReader(make([]byte, 32)),
		Enabled:        func() bool { return true },
		Server:         func() Server { return Server{"Kinosail", "server"} },
		TrustedHTTPS:   func() TrustedHTTPS { return trusted },
		SaveEnabled:    func(bool) error { return nil },
		RevokeKeys:     func() error { return nil },
		CreateKey:      func(testProfile, string) (string, error) { return "", nil },
		FindProfile:    func(string) (Profile[testProfile], bool) { return Profile[testProfile]{}, false },
		CurrentProfile: func(*http.Request) Profile[testProfile] { return Profile[testProfile]{} },
	}
	integration := &Integration[testProfile]{config: config, advertise: advertise, pairs: make(map[string]pairing[testProfile]), players: make(map[string]playerRecord), requests: make(map[string]authorization), codes: make(map[string]authorization)}
	integration.startDiscoveryLocked()
	return integration, cancel
}

func TestDiscovery(t *testing.T) { //nolint:cyclop,funlen // One matrix proves every discovery trust and lifecycle branch.
	var gotName string
	var gotPort int
	var gotText []string
	service := &testDiscovery{make(chan struct{})}
	advertise := func(name string, port int, text []string) (discovery, error) {
		gotName, gotPort, gotText = name, port, slices.Clone(text)
		return service, nil
	}
	integration, cancel := discoveryIntegration(t, "https://server.example:8443", TrustedHTTPS{Hostname: "SERVER.EXAMPLE", Configured: true}, advertise)
	if gotName != "Kinosail" || gotPort != 8443 || !slices.Equal(gotText, []string{"id=server", "version=1", "tls=true", "verify_ssl=true", "url=https://server.example:8443"}) || integration.discovery != service {
		t.Fatalf("discovery = %q %d %#v %#v", gotName, gotPort, gotText, integration.discovery)
	}
	cancel()
	select {
	case <-service.stopped:
	case <-time.After(time.Second):
		t.Fatal("lifecycle did not stop discovery")
	}

	for _, test := range []struct {
		name    string
		url     string
		port    int
		wantURL bool
	}{
		{"https default", "https://server.example", 443, true},
		{"http default", "http://server.example", 80, true},
		{"localhost", "http://localhost:8123", 8123, false},
		{"loopback", "http://127.0.0.1:8123", 8123, false},
		{"url length boundary", "https://" + string(bytes.Repeat([]byte{'x'}, 232)), 443, true},
		{"long", "https://" + string(bytes.Repeat([]byte{'x'}, 233)), 443, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var text []string
			one := func(_ string, port int, input []string) (discovery, error) {
				gotPort, text = port, slices.Clone(input)
				return &testDiscovery{make(chan struct{})}, nil
			}
			current, stop := discoveryIntegration(t, test.url, TrustedHTTPS{}, one)
			defer stop()
			if gotPort != test.port || slices.ContainsFunc(text, func(value string) bool { return strings.HasPrefix(value, "url=") }) != test.wantURL {
				t.Fatalf("discovery port/text = %d %#v", gotPort, text)
			}
			current.mu.Lock()
			current.stopDiscoveryLocked()
			current.stopDiscoveryLocked()
			current.mu.Unlock()
		})
	}

	for _, rawURL := range []string{"", "://invalid"} {
		called := false
		current, stop := discoveryIntegration(t, rawURL, TrustedHTTPS{}, func(string, int, []string) (discovery, error) { called = true; return nil, nil })
		stop()
		if called || current.discovery != nil {
			t.Errorf("invalid discovery %q advertised", rawURL)
		}
	}
	current, stop := discoveryIntegration(t, "https://server.example", TrustedHTTPS{}, func(string, int, []string) (discovery, error) { return nil, errors.New("unavailable") })
	defer stop()
	if current.discovery != nil {
		t.Fatal("failed advertisement stored")
	}
}

func TestAdvertiseHomeAssistant(t *testing.T) {
	service, err := advertiseHomeAssistant("Kinosail homeassistant package test", 38127, []string{"id=test"})
	if err != nil {
		t.Skipf("multicast DNS unavailable: %v", err)
	}
	service.Shutdown()
}
