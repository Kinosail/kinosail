package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestTMDBTokenCheckRejectsCustomAPIAddressBeforeNetworkOrPersistence(t *testing.T) {
	const token = "tmdb-read-access-token-1234567890abcdef"
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
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
	response := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": token})
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if response.Code != http.StatusConflict || requests.Load() != 0 || err != nil || loaded.String("integrations.tmdb.token") != "" {
		t.Fatalf("custom TMDB token check sent %d requests or persisted a token: status=%d err=%v", requests.Load(), response.Code, err)
	}
	fresh := t.TempDir()
	defaultConfig, err := configuration.Load(fresh, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	defaultHandler := server.New(server.Config{DataDir: fresh, RequireAuth: true, Configuration: defaultConfig})
	defaultOwner := signInTestProfile(t, defaultHandler, "/setup", "name=Owner&password=owner-password")
	change := apiCall(t, defaultHandler, defaultOwner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb.url", map[string]string{"value": provider.URL})
	unchanged, err := configuration.Load(fresh, "", func(string) (string, bool) { return "", false })
	if change.Code != http.StatusConflict || requests.Load() != 0 || err != nil || unchanged.String("integrations.tmdb.url") != "" {
		t.Fatalf("custom TMDB API address was accepted: status=%d requests=%d err=%v", change.Code, requests.Load(), err)
	}
}

func TestOwnerCanSetUpAndManageTMDBAccess(t *testing.T) { //nolint:cyclop,gocognit,funlen // One scenario proves provider validation, adapter parity, persistence, and secret handling.
	t.Parallel()
	const token = "tmdb-read-access-token-1234567890abcdef" //nolint:gosec // This is a deterministic fake token for a local test server.
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/poster.jpg" && request.Header.Get("Authorization") != "Bearer "+token {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/configuration" {
			request.Header.Set("Authorization", "Bearer token")
			fakeTMDB(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"images":{"secure_base_url":"https://image.tmdb.org/t/p/"}}`))
	}))
	defer provider.Close()
	directory := t.TempDir()
	mediaDir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.Set(directory, "integrations.tmdb.url", provider.URL); err != nil {
		t.Fatal(err)
	}
	if err := configuration.Set(directory, "integrations.tmdb.image_url", provider.URL); err != nil {
		t.Fatal(err)
	}
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: directory, CacheDir: cacheDir, RequireAuth: true, Configuration: configured, TMDBCheck: func(_ context.Context, baseURL, candidate string) error {
		if baseURL != provider.URL || candidate != token {
			return errors.New("TMDB access rejected")
		}
		return nil
	}})
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
	if !strings.Contains(saved.Body.String(), `"restartRequired":false`) {
		t.Fatalf("TMDB save still asks for a restart: %s", saved.Body.String())
	}
	imageURL := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb.image_url", map[string]string{"value": provider.URL})
	if imageURL.Code != http.StatusAccepted || !strings.Contains(imageURL.Body.String(), `"restartRequired":false`) {
		t.Fatalf("TMDB image address still asks for a restart: %d %s", imageURL.Code, imageURL.Body.String())
	}
	configurationStatus := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/configuration", nil)
	var status struct {
		Settings []struct {
			Key             string `json:"key"`
			RestartRequired bool   `json:"restartRequired"`
		} `json:"settings"`
	}
	if configurationStatus.Code != http.StatusOK || json.Unmarshal(configurationStatus.Body.Bytes(), &status) != nil {
		t.Fatalf("configuration status = %d %s", configurationStatus.Code, configurationStatus.Body.String())
	}
	for _, field := range status.Settings {
		if strings.HasPrefix(field.Key, "integrations.tmdb.") && field.RestartRequired {
			t.Fatalf("live TMDB field %s advertises a restart", field.Key)
		}
	}
	badProvider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusUnauthorized) }))
	defer badProvider.Close()
	for _, change := range []struct{ key, value string }{
		{"integrations.tmdb.url", "http://169.254.169.254/latest"},
		{"integrations.tmdb.url", "https://example.com/" + strings.Repeat("a", 2048)},
		{"integrations.tmdb.url", badProvider.URL},
		{"integrations.tmdb.image_url", "https://"},
	} {
		response := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/"+change.key, map[string]string{"value": change.value})
		loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
		if response.Code != http.StatusConflict || loadErr != nil || loaded.String("integrations.tmdb.url") != provider.URL || loaded.String("integrations.tmdb.image_url") != provider.URL {
			t.Fatalf("invalid %s changed TMDB provider: %d, err=%v", change.key, response.Code, loadErr)
		}
	}
	library := requestWithCookie(t, handler, http.MethodGet, "/", "", owner)
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(library.Body.String())
	if len(match) != 2 {
		t.Fatalf("movie missing from library: %s", library.Body.String())
	}
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		player := requestWithCookie(t, handler, http.MethodGet, "/watch/"+match[1], "", owner)
		if strings.Contains(player.Body.String(), "A linguist meets visitors.") {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatal("saving TMDB access did not start filling missing artwork without a restart")
	}
	configuredPage := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if body := configuredPage.Body.String(); configuredPage.Code != http.StatusOK || !strings.Contains(body, "TMDB access is configured.") || !strings.Contains(body, "Takes effect immediately") || strings.Contains(body, "Changes saved here apply the next time") || strings.Contains(body, token) {
		t.Fatalf("configured TMDB page = %d, secret leaked=%v", configuredPage.Code, strings.Contains(body, token))
	}
	removed := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.tmdb", nil)
	if !strings.Contains(removed.Body.String(), `"restartRequired":false`) {
		t.Fatalf("TMDB removal still asks for a restart: %s", removed.Body.String())
	}
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
	status := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/configuration", nil)
	var fields struct {
		Settings []struct {
			Key             string `json:"key"`
			RestartRequired bool   `json:"restartRequired"`
		} `json:"settings"`
	}
	if status.Code != http.StatusOK || json.Unmarshal(status.Body.Bytes(), &fields) != nil {
		t.Fatalf("managed TMDB status = %d %s", status.Code, status.Body.String())
	}
	found := false
	for _, field := range fields.Settings {
		if field.Key == "integrations.tmdb.token" {
			found = true
			if !field.RestartRequired {
				t.Fatal("managed TMDB token was advertised as live")
			}
		}
	}
	if !found {
		t.Fatal("managed TMDB token missing from configuration status")
	}
}
