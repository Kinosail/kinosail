package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func newTrustedServer(t *testing.T, config server.Config) http.Handler {
	t.Helper()
	return server.New(trustedHTTPSConfig(t, config))
}

func trustedHTTPSConfig(t *testing.T, config server.Config) server.Config {
	t.Helper()
	if config.DataDir == "" {
		config.DataDir = t.TempDir()
	}
	if err := configuration.Set(config.DataDir, "tls.duckdns", testTrustedHTTPSValue()); err != nil {
		t.Fatal(err)
	}
	configured, err := configuration.Load(config.DataDir, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	config.Configuration = configured
	return config
}

func testTrustedHTTPSValue() string {
	return `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
}

func addTestViewer(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	response := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", "name=Sam&password=viewer-password&rating=all&libraries=all", owner)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("add Viewer = %d %q", response.Code, response.Body.String())
	}
}
