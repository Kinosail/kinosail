package servertest

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type AuditFixture struct {
	New           func(string, string) http.Handler
	SignIn        func(*testing.T, http.Handler, string, string) *http.Cookie
	CookieRequest func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
	ProfileID     func(*testing.T, string, string) string
}

func (fixture AuditFixture) IdentityAuditPersistsWithoutSecrets(t *testing.T) {
	t.Helper()
	t.Parallel()
	dataDir := t.TempDir()
	handler := fixture.New("", dataDir)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	fixture.CookieRequest(t, handler, http.MethodPost, "/settings/profiles", "name=Sam&password=viewer-password", owner)
	fixture.CookieRequest(t, handler, http.MethodPost, "/settings/profiles/permissions", "id="+fixture.ProfileID(t, dataDir, "Sam")+"&rating=family", owner)
	audit, err := os.ReadFile(filepath.Join(dataDir, "audit.jsonl"))
	if err != nil || !strings.Contains(string(audit), "profile.created") || !strings.Contains(string(audit), "profile.permissions") || strings.Contains(string(audit), "owner-password") || strings.Contains(string(audit), "viewer-password") {
		t.Fatalf("audit = %q err=%v", audit, err)
	}
	system := fixture.CookieRequest(t, fixture.New("", dataDir), http.MethodGet, "/settings/system", "", owner)
	if !strings.Contains(system.Body.String(), "Recent activity") || !strings.Contains(system.Body.String(), "profile.permissions") {
		t.Fatalf("system activity = %d %q", system.Code, system.Body.String())
	}
}

func (fixture AuditFixture) OwnerActivityRecordsSettingsDenialsAndPlaybackWithoutSecrets(t *testing.T) {
	t.Helper()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(mediaDir, dataDir)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	changed := fixture.CookieRequest(t, handler, http.MethodPost, "/settings/server", "name=Cabin", owner)
	fixture.CookieRequest(t, handler, http.MethodPost, "/settings/configuration", "key=backup.key&value=correct-horse-battery-staple", owner)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/server", strings.NewReader(`{"name":"Stolen"}`)))
	home := fixture.CookieRequest(t, handler, http.MethodGet, "/", "", owner)
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	fixture.CookieRequest(t, handler, http.MethodGet, "/watch/"+id, "", owner)
	fixture.CookieRequest(t, handler, http.MethodPost, "/progress/"+id, "seconds=90&watched=true", owner)

	if changed.Header().Get("X-Request-ID") == "" || denied.Header().Get("X-Request-ID") == "" {
		t.Fatalf("request ids: changed=%q denied=%q", changed.Header().Get("X-Request-ID"), denied.Header().Get("X-Request-ID"))
	}
	activity := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/activity?limit=100", "", owner)
	for _, expected := range []string{`"action":"settings.server.updated"`, `"actor":"Owner"`, `"name":"Cabin"`, `"before.name":"Kinosail"`, `"after.name":"Cabin"`, `"key":"backup.key"`, `"value":"[redacted]"`, `"action":"access.denied"`, `"result":"denied"`, `"action":"playback.started"`, `"action":"playback.completed"`, `"title":"Arrival"`} {
		if !strings.Contains(activity.Body.String(), expected) {
			t.Fatalf("activity lacks %q: %s", expected, activity.Body.String())
		}
	}
	if strings.Contains(activity.Body.String(), "owner-password") || strings.Contains(activity.Body.String(), "correct-horse-battery-staple") || strings.Contains(activity.Body.String(), "Stolen") {
		t.Fatalf("activity exposed sensitive or denied input: %s", activity.Body.String())
	}
	fixture.assertExport(t, handler, owner)
}

func (fixture AuditFixture) ActivityJournalSurvivesRestartAndRecoveryBackup(t *testing.T) {
	t.Helper()
	dataDir := t.TempDir()
	handler := fixture.New("", dataDir)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	fixture.CookieRequest(t, handler, http.MethodPost, "/settings/server", "name=Cabin", owner)
	handler = fixture.New("", dataDir)
	activity := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/activity", "", owner)
	if !strings.Contains(activity.Body.String(), `"settings.server.updated"`) {
		t.Fatalf("restarted activity = %d %q", activity.Code, activity.Body.String())
	}
	backup := fixture.CookieRequest(t, handler, http.MethodGet, "/settings/backup", "", owner)
	gzipReader, err := gzip.NewReader(strings.NewReader(backup.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(gzipReader)
	found := false
	for {
		header, nextErr := archive.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		if header.Name == "audit.jsonl" {
			var event map[string]any
			if err := json.NewDecoder(archive).Decode(&event); err != nil || event["action"] == "" {
				t.Fatalf("backup activity event = %v, %v", event, err)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("recovery backup omitted audit.jsonl")
	}
}

func (fixture AuditFixture) assertExport(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	exported := fixture.CookieRequest(t, handler, http.MethodGet, "/api/v1/activity/export", "", owner)
	if exported.Code != http.StatusOK || exported.Header().Get("Content-Type") != "application/x-ndjson" || exported.Header().Get("Content-Disposition") == "" {
		t.Fatalf("export = %d headers=%v body=%q", exported.Code, exported.Header(), exported.Body.String())
	}
}
