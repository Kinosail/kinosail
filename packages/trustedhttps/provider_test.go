package trustedhttps

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type deSECRequest struct {
	method, path string
	record       deSECPayload
}

func deSECProviderTest(t *testing.T) (dnsProvider, *[]deSECRequest) {
	t.Helper()
	requests := make([]deSECRequest, 0, 3)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Token "+testToken || strings.Contains(request.URL.String(), testToken) {
			t.Fatal("deSEC token was not confined to the authorization header")
		}
		if request.Method == http.MethodPut && request.Header.Get("Content-Type") != "application/json" || request.Method == http.MethodDelete && request.Header.Get("Content-Type") != "" {
			t.Fatalf("content type for %s = %q", request.Method, request.Header.Get("Content-Type"))
		}
		recorded := deSECRequest{method: request.Method, path: request.URL.Path}
		if request.Method == http.MethodPut {
			if err := json.NewDecoder(request.Body).Decode(&recorded.record); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, recorded)
		writer.WriteHeader(map[string]int{http.MethodPut: http.StatusOK, http.MethodDelete: http.StatusNoContent}[request.Method])
	}))
	t.Cleanup(server.Close)
	deSEC, err := newDNSProvider(Config{Provider: ProviderDeSEC, Domain: "family.dedyn.io", Token: testToken, Address: "server.nox", Terms: true}, server.Client(), providerEndpoints{deSEC: server.URL, lookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("192.168.1.10")}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return deSEC, &requests
}

func TestDeSECProviderUpdatesAddressWithPrivateAuthorization(t *testing.T) {
	t.Parallel()
	deSEC, requests := deSECProviderTest(t)
	if err := deSEC.UpdateAddress(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := *requests
	if len(got) != 1 || got[0].method != http.MethodPut || got[0].path != "/api/v1/domains/family.dedyn.io/rrsets/@/A/" || got[0].record.Subname != "" || got[0].record.Type != "A" || got[0].record.TTL != 3600 || strings.Join(got[0].record.Records, "") != "192.168.1.10" {
		t.Fatalf("address request = %#v", got)
	}
}

func TestDeSECProviderManagesTXTWithPrivateAuthorization(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the provider boundary matrix.
	t.Parallel()
	deSEC, requests := deSECProviderTest(t)
	if err := deSEC.PresentTXT(t.Context(), "dns-proof"); err != nil {
		t.Fatal(err)
	}
	if err := deSEC.CleanupTXT(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := *requests
	path := "/api/v1/domains/family.dedyn.io/rrsets/_acme-challenge/TXT/"
	if len(got) != 2 || got[0].method != http.MethodPut || got[0].path != path || got[0].record.Subname != "_acme-challenge" || got[0].record.Type != "TXT" || got[0].record.TTL != 3600 || strings.Join(got[0].record.Records, "") != `"dns-proof"` || got[1].method != http.MethodDelete || got[1].path != path {
		t.Fatalf("TXT requests = %#v", got)
	}
}

func TestDuckDNSProviderResolvesLocalHostnameBeforeARecordUpdate(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("ip"); got != "192.168.1.42" {
			t.Fatalf("DuckDNS ip = %q, want 192.168.1.42", got)
		}
		_, _ = writer.Write([]byte("OK"))
	}))
	duck, err := newDNSProvider(Config{Provider: ProviderDuckDNS, Domain: "family", Token: testToken, Address: "server.nox", Terms: true}, server.Client(), providerEndpoints{
		duckDNS:  server.URL,
		lookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("192.168.1.42")}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = duck.UpdateAddress(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckUpdatesTheConfiguredAddress(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Query().Get("domains") != "family" || request.URL.Query().Get("ip") != "192.168.1.10" || request.URL.Query().Get("token") != testToken {
			t.Fatalf("connection test query = %q", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte("OK"))
	}))
	config := Config{Provider: ProviderDuckDNS, Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	if err := Check(t.Context(), config, Dependencies{Client: server.Client(), UpdateURL: server.URL}); err != nil || requests != 1 {
		t.Fatalf("Check() error = %v, requests = %d", err, requests)
	}
}

func TestDuckDNSProviderRejectsUnsafeHostnameResolutionBeforeRequest(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = writer.Write([]byte("OK"))
	}))
	duck, err := newDNSProvider(Config{Provider: ProviderDuckDNS, Domain: "family", Token: testToken, Address: "server.nox", Terms: true}, server.Client(), providerEndpoints{
		duckDNS:  server.URL,
		lookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.2")}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = duck.UpdateAddress(t.Context()); err == nil || requests != 0 {
		t.Fatalf("unsafe hostname update error = %v, requests = %d", err, requests)
	}
}

func TestDuckDNSProviderPreservesExistingAPI(t *testing.T) {
	t.Parallel()
	queries := make([]string, 0, 3)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries = append(queries, request.URL.Query().Encode())
		_, _ = writer.Write([]byte("OK"))
	}))
	duck, err := newDNSProvider(Config{Provider: ProviderDuckDNS, Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}, server.Client(), providerEndpoints{duckDNS: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{
		func() error { return duck.UpdateAddress(t.Context()) },
		func() error { return duck.PresentTXT(t.Context(), "dns-proof") },
		func() error { return duck.CleanupTXT(t.Context()) },
	} {
		if err = operation(); err != nil {
			t.Fatal(err)
		}
	}
	if len(queries) != 3 || !strings.Contains(queries[0], "ip=192.168.1.10") || !strings.Contains(queries[1], "txt=dns-proof") || !strings.Contains(queries[2], "clear=true") {
		t.Fatalf("queries = %v", queries)
	}
}

func TestProviderErrorsNeverExposeToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, testToken, http.StatusUnauthorized)
	}))
	provider, err := newDNSProvider(Config{Provider: ProviderDeSEC, Domain: "family.dedyn.io", Token: testToken, Address: "192.168.1.10", Terms: true}, server.Client(), providerEndpoints{deSEC: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err = provider.UpdateAddress(t.Context()); err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("unsafe provider error = %v", err)
	}
}
