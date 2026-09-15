package remoteaccess

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCredentialLengthBoundaries(t *testing.T) {
	for _, config := range []Config{
		{Domain: strings.Repeat("a", 63), Token: strings.Repeat("A", 32)},
		{Domain: " Family-2 ", Token: strings.Repeat("z", 128)},
	} {
		validated, err := validateConfig(config)
		if err != nil {
			t.Fatalf("boundary config rejected: %v", err)
		}
		if validated.Domain != strings.ToLower(strings.TrimSpace(config.Domain)) || validated.Token != config.Token {
			t.Fatalf("validated config = %#v", validated)
		}
	}
}

func TestOversizedCredentialsFailBeforeFilesystemInitialization(t *testing.T) {
	for name, credentials := range map[string]struct{ domain, token string }{
		"domain": {strings.Repeat("a", 64), strings.Repeat("b", 32)},
		"token":  {"family", strings.Repeat("b", 129)},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			config := Config{Enabled: true, PublicHTTPS: true, Domain: credentials.domain, Token: credentials.token, Listen: "127.0.0.1:8443", DataDir: directory}
			manager, err := New(config)
			if err == nil || manager != nil {
				t.Fatalf("oversized credentials accepted: manager=%#v error=%v", manager, err)
			}
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("rejected credentials created %d file entries", len(entries))
			}
		})
	}
}

func TestPublicHostRejectsMalformedAndOversizedValuesWithoutHandlerSideEffects(t *testing.T) {
	handled := 0
	handler := exactPublicHost("family.duckdns.org", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { handled++ }))
	for name, host := range map[string]string{
		"missing":              "",
		"wrong port":           "family.duckdns.org:8443",
		"untrusted suffix":     "family.duckdns.org.evil",
		"multiple final dots":  "family.duckdns.org..",
		"oversized host value": strings.Repeat("a", 256),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://family.duckdns.org/", nil)
			request.Host = host
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusMisdirectedRequest || handled != 0 {
				t.Fatalf("response=%d handled=%d", response.Code, handled)
			}
		})
	}
	for _, host := range []string{"family.duckdns.org", "FAMILY.DUCKDNS.ORG.", "family.duckdns.org:443"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://family.duckdns.org/", nil)
		request.Host = host
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	if handled != 3 {
		t.Fatalf("accepted hosts handled = %d", handled)
	}
}
