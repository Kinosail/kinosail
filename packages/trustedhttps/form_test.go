package trustedhttps

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseSettingsForm(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"", "duckdns", "desec"} {
		form := url.Values{"domain": {"house"}, "token": {""}, "address": {"192.168.1.2"}, "termsAccepted": {"true"}, "_csrf": {"form-token"}}
		if provider != "" {
			form.Set("provider", provider)
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		got, err := ParseSettingsForm(httptest.NewRecorder(), request)
		want := SettingsInput{Provider: provider, Domain: "house", Address: "192.168.1.2", TermsAccepted: true}
		if err != nil || got != want {
			t.Fatalf("parsed form = %#v, %v; want %#v", got, err, want)
		}
	}
}

func TestParseSettingsFormRejectsInvalidTransportAndFields(t *testing.T) {
	t.Parallel()
	valid := "domain=house&token=private&address=192.168.1.2&termsAccepted=true"
	for name, body := range map[string]string{
		"missing domain":  "token=x&address=x&termsAccepted=true",
		"missing token":   "domain=x&address=x&termsAccepted=true",
		"missing address": "domain=x&token=x&termsAccepted=true",
		"missing terms":   "domain=x&token=x&address=x",
		"declined terms":  "domain=x&token=x&address=x&termsAccepted=false",
		"duplicate":       valid + "&domain=other",
		"unknown":         valid + "&extra=x",
		"malformed":       valid + "&provider=%zz",
		"oversized":       valid + "&token=" + strings.Repeat("x", 16<<10),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			got, err := ParseSettingsForm(httptest.NewRecorder(), request)
			if err == nil || got != (SettingsInput{}) {
				t.Fatalf("invalid form returned usable input: %#v, %v", got, err)
			}
		})
	}
	for _, target := range []string{"/settings", "/settings?domain=other"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(valid))
		if target != "/settings" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if _, err := ParseSettingsForm(httptest.NewRecorder(), request); err == nil {
			t.Fatalf("invalid transport accepted for %s", target)
		}
	}
}
