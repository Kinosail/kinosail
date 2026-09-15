package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOwnerCanSetUpAndManageTMDBAccess(t *testing.T) { //nolint:cyclop,gocognit,funlen // One scenario proves provider validation, adapter parity, persistence, and secret handling.
	t.Parallel()
	const token = "tmdb-read-access-token-1234567890abcdef" //nolint:gosec // This is a deterministic fake token for a local test server.
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/configuration" || request.Header.Get("Authorization") != "Bearer "+token {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"images":{"secure_base_url":"https://image.tmdb.org/t/p/"}}`))
	}))
	defer provider.Close()
	directory := t.TempDir()
	if err := configuration.Set(directory, "integrations.tmdb.url", provider.URL); err != nil {
		t.Fatal(err)
	}
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	onboarding := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	settingsPage := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	configurationPage := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	for name, response := range map[string]*httptest.ResponseRecorder{"onboarding": onboarding, "settings": settingsPage, "configuration": configurationPage} {
		if response.Code != http.StatusOK {
			t.Fatalf("%s page = %d", name, response.Code)
		}
	}
	assertAPIBody(t, onboarding, http.StatusOK, `id="integrations.tmdb"`, `action="/onboarding/tmdb"`, "Create a free TMDB account", `href="https://www.themoviedb.org/login"`, "Request API access", "API Read Access Token")
	assertAPIBody(t, settingsPage, http.StatusOK, `href="/settings/configuration#integrations.tmdb"`, "Set up or manage access")
	assertAPIBody(t, configurationPage, http.StatusOK, `id="integrations.tmdb"`, `action="/settings/configuration"`, `aria-label="TMDB API Read Access Token"`, "Kinosail checks the token before saving it")

	short := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": "short"})
	unknown := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": token, "unknown": "true"})
	rejected := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": strings.Repeat("x", 40)})
	individual := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb.token", map[string]string{"value": token})
	if short.Code != http.StatusBadRequest || unknown.Code != http.StatusBadRequest || rejected.Code != http.StatusConflict || individual.Code != http.StatusConflict {
		t.Fatalf("invalid TMDB access: short=%d unknown=%d rejected=%d individual=%d", short.Code, unknown.Code, rejected.Code, individual.Code)
	}
	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": token})
	loaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusAccepted || loadErr != nil || loaded.String("integrations.tmdb.token") != token {
		t.Fatalf("TMDB API save = %d, configured=%v, err=%v", saved.Code, loaded.Public("integrations.tmdb.token").Configured, loadErr)
	}
	configuredPage := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if body := configuredPage.Body.String(); configuredPage.Code != http.StatusOK || !strings.Contains(body, "TMDB access is configured.") || strings.Contains(body, token) {
		t.Fatalf("configured TMDB page = %d, secret leaked=%v", configuredPage.Code, strings.Contains(body, token))
	}
	removed := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.tmdb", nil)
	webSaved := webFormCall(t, handler, owner.Value, "/onboarding/tmdb", map[string][]string{"key": {"integrations.tmdb"}, "token": {token}})
	loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if removed.Code != http.StatusAccepted || webSaved.Code != http.StatusSeeOther || webSaved.Header().Get("Location") != "/onboarding/connection#integrations.tmdb" || loadErr != nil || loaded.String("integrations.tmdb.token") != token {
		t.Fatalf("TMDB reset/web save: removed=%d saved=%d redirect=%q configured=%v err=%v", removed.Code, webSaved.Code, webSaved.Header().Get("Location"), loaded.Public("integrations.tmdb.token").Configured, loadErr)
	}

	ambiguous := webFormCall(t, handler, owner.Value, "/onboarding/tmdb", map[string][]string{"key": {"integrations.tmdb"}, "token": {token, token}})
	unknownWeb := webFormCall(t, handler, owner.Value, "/onboarding/tmdb", map[string][]string{"key": {"integrations.tmdb"}, "token": {token}, "unknown": {"true"}})
	loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if ambiguous.Code != http.StatusBadRequest || unknownWeb.Code != http.StatusBadRequest || loadErr != nil || loaded.String("integrations.tmdb.token") != token {
		t.Fatalf("invalid TMDB web forms changed state: ambiguous=%d unknown=%d configured=%v err=%v", ambiguous.Code, unknownWeb.Code, loaded.Public("integrations.tmdb.token").Configured, loadErr)
	}
}

func TestManagedTMDBAccessIsReadOnlyAndSecret(t *testing.T) {
	t.Parallel()
	const token = "managed-tmdb-read-access-token-1234567890" //nolint:gosec // This is a deterministic fake managed token for a local test.
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(key string) (string, bool) {
		if key == "KINOSAIL_TMDB_TOKEN" {
			return token, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, "KINOSAIL_TMDB_TOKEN") || strings.Contains(body, token) || strings.Contains(body, `aria-label="TMDB API Read Access Token"`) {
		t.Fatalf("managed TMDB page = %d, secret leaked=%v, editable=%v", page.Code, strings.Contains(body, token), strings.Contains(body, `aria-label="TMDB API Read Access Token"`))
	}
	changed := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": strings.Repeat("x", 40)})
	if changed.Code != http.StatusConflict {
		t.Fatalf("managed TMDB update = %d", changed.Code)
	}
}
