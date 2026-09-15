package trustedhttps

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckAndProviderConstructionRejectInvalidBoundaries(t *testing.T) {
	if err := Check(t.Context(), Config{Domain: "family"}); err == nil {
		t.Fatal("invalid check configuration accepted")
	}
	config := Config{Provider: ProviderDuckDNS, Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	for name, endpoints := range map[string]providerEndpoints{
		"duckdns": {duckDNS: "%"},
		"desec":   {deSEC: "%"},
	} {
		t.Run(name, func(t *testing.T) {
			input := config
			if name == "desec" {
				input.Provider, input.Domain = ProviderDeSEC, "family.dedyn.io"
			}
			if _, err := newDNSProvider(input, http.DefaultClient, endpoints); err == nil {
				t.Fatal("invalid provider endpoint accepted")
			}
		})
	}
	config.Provider = "unsupported"
	if _, err := newDNSProvider(config, http.DefaultClient, providerEndpoints{}); err == nil {
		t.Fatal("unsupported provider accepted")
	}
	if err := Check(t.Context(), Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}, Dependencies{UpdateURL: "%"}); err == nil {
		t.Fatal("invalid Check endpoint accepted")
	}
}

func TestProviderDefaultsIncludeResolver(t *testing.T) {
	defaults := (providerEndpoints{}).defaults()
	if defaults.duckDNS != duckDNSUpdateURL || defaults.deSEC != deSECAPIURL || defaults.lookupIP == nil {
		t.Fatalf("provider defaults = %#v", defaults)
	}
	addresses, err := defaults.lookupIP(t.Context(), "localhost")
	if err != nil || len(addresses) == 0 {
		t.Fatalf("localhost resolution = %v, %v", addresses, err)
	}
	client := defaultHTTPClient()
	if client.Timeout != 15*time.Second {
		t.Fatalf("provider timeout = %v", client.Timeout)
	}
	if err = client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy = %v", err)
	}
}

func TestDeSECProviderRejectsResolutionAndAcceptsCleanupNotFound(t *testing.T) {
	config := Config{Provider: ProviderDeSEC, Domain: "family.dedyn.io", Token: testToken, Address: "server.nox", Terms: true}
	provider, err := newDNSProvider(config, http.DefaultClient, providerEndpoints{lookupIP: func(context.Context, string) ([]net.IP, error) {
		return nil, errors.New("resolver unavailable")
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = provider.UpdateAddress(t.Context()); err == nil {
		t.Fatal("resolution failure accepted")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	provider, err = newDNSProvider(Config{Provider: ProviderDeSEC, Domain: "family.dedyn.io", Token: testToken, Address: "192.168.1.10", Terms: true}, server.Client(), providerEndpoints{deSEC: server.URL})
	if err != nil || provider.CleanupTXT(t.Context()) != nil {
		t.Fatalf("idempotent cleanup = %v", err)
	}
}

func TestProviderResponseHandlesTransportBodyAndStatusFailures(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://provider.example", nil)
	for name, client := range map[string]*http.Client{
		"transport": {Transport: providerTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })},
		"body": {Transport: providerTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: providerFailingBody{}}, nil
		})},
		"status": {Transport: providerTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("no"))}, nil
		})},
	} {
		t.Run(name, func(t *testing.T) {
			if err := providerResponse(client, request, "provider", http.StatusOK); err == nil {
				t.Fatal("provider failure accepted")
			}
		})
	}
}

func TestDeSECPayloadFailureStopsBeforeRequest(t *testing.T) {
	config := Config{Provider: ProviderDeSEC, Domain: "family.dedyn.io", Token: testToken, Address: "192.168.1.10", Terms: true}
	provider, err := newDNSProvider(config, http.DefaultClient, providerEndpoints{})
	if err != nil {
		t.Fatal(err)
	}
	deSEC := provider.(*deSECProvider)
	deSEC.marshal = func(deSECPayload) ([]byte, error) { return nil, errors.New("encode failed") }
	if err = deSEC.PresentTXT(t.Context(), "proof"); err == nil {
		t.Fatal("payload encoding failure accepted")
	}
}

type providerTransport func(*http.Request) (*http.Response, error)

func (transport providerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type providerFailingBody struct{}

func (providerFailingBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (providerFailingBody) Close() error             { return nil }
