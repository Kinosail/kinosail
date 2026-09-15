package metadata

import (
	"bytes"
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkBoundaryValidation(t *testing.T) { //nolint:cyclop // The assertions cover one network trust boundary.
	t.Parallel()
	for _, raw := range []string{"8.8.8.8", "2001:4860:4860::8888"} {
		if !allowedOutboundIP(net.ParseIP(raw)) {
			t.Fatalf("public address %q rejected", raw)
		}
	}
	for _, raw := range []string{"", "0.0.0.0", "127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "224.0.0.1"} {
		if allowedOutboundIP(net.ParseIP(raw)) {
			t.Fatalf("prohibited address %q accepted", raw)
		}
	}
	if !cgnatIP(net.ParseIP("100.127.255.255")) || cgnatIP(net.ParseIP("100.128.0.1")) || cgnatIP(net.ParseIP("::1")) {
		t.Fatal("CGNAT classification mismatch")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if finite(value) {
			t.Fatalf("non-finite value %v accepted", value)
		}
	}
	if !finite(1) {
		t.Fatal("finite number rejected")
	}
}

func TestProviderEndpointValidation(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"https://api.example.com", "http://localhost:1234", "http://127.0.0.1:1234"} {
		endpoint, _ := url.Parse(raw)
		if !validProviderEndpoint(endpoint, "api.example.com", "") {
			t.Fatalf("valid endpoint %q rejected", raw)
		}
	}
	for _, raw := range []string{"http://api.example.com", "https://user@api.example.com", "https://api.example.com:443", "https://api.example.com/path", "https://api.example.com?x=1", "https://api.example.com#x", "http://localhost"} {
		endpoint, _ := url.Parse(raw)
		if validProviderEndpoint(endpoint, "api.example.com", "") {
			t.Fatalf("invalid endpoint %q accepted", raw)
		}
	}
	if validProviderEndpoint(nil, "api.example.com", "") {
		t.Fatal("nil endpoint accepted")
	}
	for _, raw := range []string{"https://api.example.com/v1", "http://localhost:1234/v1", "http://127.0.0.1:1234"} {
		if !validProviderBaseURL(raw) {
			t.Fatalf("valid base URL %q rejected", raw)
		}
	}
	for _, raw := range []string{"", " https://api.example.com", "http://api.example.com", "https://user@api.example.com", "https://api.example.com/a/../b", "https://api.example.com?x=1", "http://localhost/path"} {
		if validProviderBaseURL(raw) {
			t.Fatalf("invalid base URL %q accepted", raw)
		}
	}
}

func TestExternalJSONAndAtomicSave(t *testing.T) {
	t.Parallel()
	var target struct{ Value string }
	if err := decodeExternalJSONStrict(strings.NewReader(`{"Value":"ok"}`), 64, &target); err != nil || target.Value != "ok" {
		t.Fatalf("strict JSON = %#v, %v", target, err)
	}
	if err := decodeExternalJSONStrict(strings.NewReader(`{"Value":"ok","unknown":true}`), 64, &target); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
	if err := decodeExternalJSON(strings.NewReader(`{"Value":"ok","unknown":true}`), 64, &target); err != nil {
		t.Fatalf("provider extension field rejected: %v", err)
	}
	if err := decodeExternalJSON(strings.NewReader(`{"Value":"`+strings.Repeat("x", 100)+`"}`), 32, &target); err == nil {
		t.Fatal("oversized JSON accepted")
	}
	path := filepath.Join(t.TempDir(), "nested", "value.json")
	if err := saveJSON(path, map[string]string{"value": "ok"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(`"ok"`)) {
		t.Fatalf("saved JSON = %q, %v", data, err)
	}
	if err := saveJSON(path, make(chan int)); err == nil {
		t.Fatal("unsupported JSON value accepted")
	}
}

func TestOutboundClientRejectsMalformedAddressAndRedirect(t *testing.T) {
	t.Parallel()
	client := outboundHTTPClient(0, allowedIntegrationIP)
	transport := client.Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", "missing-port"); err == nil {
		t.Fatal("malformed outbound address accepted")
	}
	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", nil)
	if err := client.CheckRedirect(request, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect error = %v", err)
	}
}
