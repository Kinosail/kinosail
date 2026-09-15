package server_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func newJellyfinServer(t *testing.T, config server.Config) http.Handler {
	t.Helper()
	config = trustedJellyfinConfig(t, config)
	settings := filepath.Join(config.DataDir, "settings.json")
	value := map[string]any{"name": "Kinosail", "libraries": []string{"."}}
	if data, err := os.ReadFile(settings); err == nil {
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	value["jellyfinCompatibility"] = true
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return server.New(config)
}

func trustedJellyfinConfig(t *testing.T, config server.Config) server.Config {
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

func enableJellyfin(t *testing.T, handler http.Handler, ownerToken string) {
	t.Helper()
	assertAPIBody(t, apiCall(t, handler, ownerToken, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family-media", "token": strings.Repeat("t", 32), "address": "192.168.1.10", "termsAccepted": true}), http.StatusAccepted, `"restartRequired":true`)
	assertAPIBody(t, apiCall(t, handler, ownerToken, http.MethodPut, "/api/v1/settings/jellyfin", map[string]bool{"enabled": true}), http.StatusOK, `"status":"saved"`)
}

func testTrustedHTTPSValue() string {
	return `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
}
