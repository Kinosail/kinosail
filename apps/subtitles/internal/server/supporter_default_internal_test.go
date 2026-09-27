package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type supporterRoundTrip func(*http.Request) (*http.Response, error)

func (roundTrip supporterRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestDefaultSupporterRejectsAnUnknownSigningKeyWithoutSaving(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	client := &http.Client{Transport: supporterRoundTrip(func(request *http.Request) (*http.Response, error) {
		called = true
		const want = "https://kinosail-supporter-prod.pvw-7m4q2x9.workers.dev/v1/supporters/activate"
		if request.Method != http.MethodPost || request.URL.String() != want {
			t.Errorf("default supporter request = %s %s, want POST %s", request.Method, request.URL, want)
		}
		body := fmt.Sprintf(`{"certificate":"AA","signature":"AA","publicKey":%q,"activationId":"00000000-0000-4000-8000-000000000010"}`, base64.RawURLEncoding.EncodeToString(public))
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}
	settings := &settingsStore{}
	program := newSupporterProgram(settings, SupporterConfig{HTTPClient: client})
	if err := program.activate(t.Context(), "VALID_SUPPORTER_KEY", ""); err == nil || err.Error() != "supporter activation returned an invalid response" {
		t.Fatalf("unknown signing key = %v, want invalid response", err)
	}
	if !called || settings.value.Supporter != nil {
		t.Fatalf("unknown signing key contacted provider=%t and saved supporter=%t", called, settings.value.Supporter != nil)
	}
}

func TestDefaultSupporterLinksToBothPolarCheckoutFamilies(t *testing.T) {
	program := newSupporterProgram(&settingsStore{}, SupporterConfig{})
	page := program.pageData("")
	if page.LivingStandard.SupportURL != "https://buy.polar.sh/polar_cl_HTsVz4n50S840QL5zKCZ0yTZe6HpVfHqagCtQ1LZ6sN" {
		t.Errorf("monthly checkout = %q", page.LivingStandard.SupportURL)
	}
	if page.PatronOrder.SupportURL != "https://buy.polar.sh/polar_cl_Aec8B5u63m6Wr6l6bfZNMS6dmYRyM4jAMAzxQ28S4b3" {
		t.Errorf("one-time checkout = %q", page.PatronOrder.SupportURL)
	}
}

func TestConfiguredSupporterCheckoutURLStillOverridesBothFamilies(t *testing.T) {
	program := newSupporterProgram(&settingsStore{}, SupporterConfig{SupportURL: "https://example.com/support"})
	page := program.pageData("")
	if page.LivingStandard.SupportURL != "https://example.com/support" || page.PatronOrder.SupportURL != "https://example.com/support" {
		t.Fatalf("configured checkout links = %q and %q", page.LivingStandard.SupportURL, page.PatronOrder.SupportURL)
	}
}
