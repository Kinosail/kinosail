package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerSeesOnlyUnappliedRestartChanges(t *testing.T) {
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	assertRestartPages(t, handler, owner, false)
	change := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/backup.interval", map[string]string{"value": "12h"})
	if change.Code != http.StatusAccepted || !strings.Contains(change.Body.String(), `"restartRequired":true`) {
		t.Fatalf("save = %d %s", change.Code, change.Body.String())
	}
	assertRestartPages(t, handler, owner, true)
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	restarted := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: loaded})
	assertRestartPages(t, restarted, owner, false)
	invalid := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/backup.interval", map[string]string{"value": "invalid"})
	if invalid.Code == http.StatusAccepted {
		t.Fatal("invalid interval was accepted")
	}
	assertRestartPages(t, handler, owner, true)
	reset := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/backup.interval", nil)
	if reset.Code != http.StatusAccepted || !strings.Contains(reset.Body.String(), `"restartRequired":false`) {
		t.Fatalf("reset = %d %s", reset.Code, reset.Body.String())
	}
	assertRestartPages(t, handler, owner, false)
}

func assertRestartPages(t *testing.T, handler http.Handler, owner *http.Cookie, want bool) {
	t.Helper()
	for _, path := range []string{"/settings", "/settings/configuration"} {
		page := requestWithCookie(t, handler, http.MethodGet, path, "", owner)
		if page.Code != http.StatusOK || strings.Contains(page.Body.String(), `id="restart-required"`) != want {
			t.Fatalf("%s restart status = %d, want visible=%t", path, page.Code, want)
		}
		if want && (!strings.Contains(page.Body.String(), "Backup frequency") || !strings.Contains(page.Body.String(), "Restart Kinosail Subtitles Server")) {
			t.Fatalf("%s lacks actionable restart status", path)
		}
	}
}

func TestPendingRestartNoticeInPhoneAndDesktopBrowsers(t *testing.T) {
	chrome := chromeExecutable(t)
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/backup.interval", map[string]string{"value": "12h"}); saved.Code != http.StatusAccepted {
		t.Fatalf("save = %d", saved.Code)
	}
	web := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.AddCookie(owner)
		handler.ServeHTTP(writer, request)
	}))
	defer web.Close()
	for _, path := range []string{"/settings", "/settings/configuration"} {
		for _, width := range []int{390, 1280} {
			result := browserAudit(t, chrome, web.URL+path, width, 900)
			if result.Overflow != 0 || result.Main != 1 || result.Unnamed != 0 {
				t.Fatalf("%s at %dpx = %+v", path, width, result)
			}
		}
	}
}
