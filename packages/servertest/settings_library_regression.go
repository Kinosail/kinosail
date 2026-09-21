package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (suite SettingsRegression) OwnerCanOpenLibrarySettings(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: t.TempDir()}).ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Library folders") || !strings.Contains(response.Body.String(), mediaDir) || !strings.Contains(response.Body.String(), suite.ProtectionText) || !strings.Contains(response.Body.String(), "backup key") {
		t.Fatalf("settings = %d %q", response.Code, response.Body.String())
	}
}

func (suite SettingsRegression) SettingsShowLibraryDiagnostics(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{"Arrival.mp4", "Heat.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: t.TempDir()}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/system", nil))

	if !strings.Contains(response.Body.String(), "Diagnostics") || !strings.Contains(response.Body.String(), "Library items: 2") || !strings.Contains(response.Body.String(), "Library monitoring") {
		t.Fatalf("settings = %q", response.Body.String())
	}
}

func (suite SettingsRegression) OwnerCanPersistALibraryFolder(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	for _, path := range []string{"Movies/Arrival.mp4", "TV/Severance.S01E01.mkv"} {
		full := filepath.Join(mediaDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/libraries", strings.NewReader(url.Values{"path": {"Movies"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	home := httptest.NewRecorder()
	suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir}).ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(home.Body.String(), "Arrival") || strings.Contains(home.Body.String(), "Severance") {
		t.Fatalf("save = %d, home = %q", response.Code, home.Body.String())
	}
}

func (suite SettingsRegression) LibraryFolderCannotEscapeMediaMount(t *testing.T) {
	t.Parallel()

	mediaDir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(mediaDir, "Escape")); err != nil {
		t.Fatal(err)
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/libraries", strings.NewReader("path=Escape"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("escape = %d %q", response.Code, response.Body.String())
	}
}

func (suite SettingsRegression) LibrarySettingPathsRejectAmbiguousInputWithoutSideEffects(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: t.TempDir()})
	for name, path := range map[string]string{
		"missing":   "",
		"oversized": strings.Repeat("x", 4097),
		"parent":    "../outside",
		"absolute":  filepath.Join(mediaDir, "Movies"),
	} {
		t.Run(name, func(t *testing.T) {
			for _, route := range []string{"/settings/libraries", "/settings/libraries/remove"} {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, route, strings.NewReader(url.Values{"path": {path}}.Encode()))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusBadRequest {
					t.Fatalf("POST %s with %s path = %d %q", route, name, response.Code, response.Body.String())
				}
			}
			for _, method := range []string{http.MethodPost, http.MethodDelete} {
				response := suite.APICall(t, handler, "", method, "/api/v1/libraries", map[string]string{"path": path})
				if response.Code != http.StatusBadRequest {
					t.Fatalf("%s /api/v1/libraries with %s path = %d %q", method, name, response.Code, response.Body.String())
				}
			}
		})
	}
	suite.AssertBody(t, suite.APICall(t, handler, "", http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"libraries":["."]`)
}

func (suite SettingsRegression) OwnerCanRemoveALibraryFolder(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(mediaDir, "Movies"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/libraries/remove", strings.NewReader("path=."))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if response.Code != http.StatusSeeOther || strings.Contains(settings.Body.String(), `action="/settings/libraries/remove" method="post"><code>.</code>`) {
		t.Fatalf("remove = %d, settings = %q", response.Code, settings.Body.String())
	}
}
