package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func setupWithLanguage(t *testing.T, acceptLanguage string) []string {
	t.Helper()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	body, _ := json.Marshal(map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", acceptLanguage)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var session struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &session) != nil {
		t.Fatalf("setup = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	settings := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil)
	var values struct {
		Languages []string `json:"subtitleLanguages"`
	}
	if settings.Code != http.StatusOK || json.Unmarshal(settings.Body.Bytes(), &values) != nil {
		t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
	}
	return values.Languages
}

func TestFirstSetupPrefersEnglishPlusTheOwnersInterfaceLanguage(t *testing.T) {
	t.Parallel()
	for acceptLanguage, want := range map[string][]string{
		"de-DE,de;q=0.9,en;q=0.5": {"en", "de"},
		"pt-BR,pt;q=0.9":          {"en", "pt-BR"},
		"en-US,en;q=0.9":          {"en"},
		"":                        {"en"},
	} {
		if got := setupWithLanguage(t, acceptLanguage); !slices.Equal(got, want) {
			t.Errorf("Accept-Language %q: subtitle languages = %v, want %v", acceptLanguage, got, want)
		}
	}
}
