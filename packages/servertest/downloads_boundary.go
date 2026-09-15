package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DownloadFixture connects download contracts to each app's real account and media fixtures.
type DownloadFixture struct {
	NewHandler          func(media, data, cache string) http.Handler
	NewTranscodeHandler func(media, data, cache, ffmpeg string) http.Handler
	DownloadsScript     string
	APIServer           func(*testing.T) (http.Handler, string)
	FirstItemID         func(*testing.T, http.Handler, string) string
	SignIn              func(*testing.T, http.Handler, string, string) *http.Cookie
	APICall             func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
	CookieRequest       func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
}

// DownloadsBoundaryContract runs each download scenario against the real app.
func DownloadsBoundaryContract(t *testing.T, fixture DownloadFixture) {
	t.Helper()
	for _, test := range []struct {
		name string
		run  func(*testing.T, DownloadFixture)
	}{
		{"OwnerCanCreateIdempotentOriginalDownloadAndRemoveIt", OwnerCanCreateIdempotentOriginalDownloadAndRemoveIt},
		{"OfflineDownloadValidationIsPublicThroughAPIAndWeb", OfflineDownloadValidationIsPublicThroughAPIAndWeb},
		{"OfflineDownloadRejectsTrailingJSONBeforeCreatingAJob", OfflineDownloadRejectsTrailingJSONBeforeCreatingAJob},
	} {
		t.Run(test.name, func(t *testing.T) { test.run(t, fixture) })
	}
}

// OwnerCanCreateIdempotentOriginalDownloadAndRemoveIt runs the shared download regression contract.
func OwnerCanCreateIdempotentOriginalDownloadAndRemoveIt(t *testing.T, fixture DownloadFixture) {
	media, data, cache := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("original-media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, data, cache)
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token := cookie.Value
	itemID := fixture.FirstItemID(t, handler, token)
	started := fixture.APICall(t, handler, token, http.MethodPost, "/api/v1/items/"+itemID+"/downloads", map[string]any{"quality": "original"})
	var job struct{ ID string }
	if err := json.Unmarshal(started.Body.Bytes(), &job); err != nil {
		t.Fatalf("JSON = %d %q: %v", started.Code, started.Body.String(), err)
	}
	if started.Code != http.StatusAccepted || job.ID == "" {
		t.Fatalf("start = %d %q", started.Code, started.Body.String())
	}
	if repeated := fixture.APICall(t, handler, token, http.MethodPost, "/api/v1/items/"+itemID+"/downloads", map[string]any{"quality": "original"}); repeated.Code != http.StatusAccepted || !containsJSONID(repeated.Body.String(), job.ID) {
		t.Fatalf("repeat = %d %q", repeated.Code, repeated.Body.String())
	}
	assertOriginalDownloadReady(t, fixture, handler, token, job.ID)
	page := fixture.CookieRequest(t, handler, http.MethodGet, "/offline-downloads", "", cookie)
	if page.Code != http.StatusOK {
		t.Fatalf("downloads page = %d %q", page.Code, page.Body.String())
	}
	removed := fixture.CookieRequest(t, handler, http.MethodPost, "/offline-downloads/"+job.ID+"/remove", "", cookie)
	if removed.Code != http.StatusSeeOther {
		t.Fatalf("web remove = %d %q", removed.Code, removed.Body.String())
	}
	if missing := fixture.APICall(t, handler, token, http.MethodDelete, "/api/v1/downloads/"+job.ID, nil); missing.Code != http.StatusNotFound {
		t.Fatalf("removed download = %d %q", missing.Code, missing.Body.String())
	}
}

// OfflineDownloadValidationIsPublicThroughAPIAndWeb runs the shared download regression contract.
func OfflineDownloadValidationIsPublicThroughAPIAndWeb(t *testing.T, fixture DownloadFixture) {
	handler, token := fixture.APIServer(t)
	itemID := fixture.FirstItemID(t, handler, token)
	for _, quality := range []string{"original", "invalid", "audio"} {
		response := fixture.APICall(t, handler, token, http.MethodPost, "/api/v1/items/"+itemID+"/downloads", map[string]any{"quality": quality})
		if response.Code != http.StatusBadRequest {
			t.Fatalf("quality %q without cache = %d %q", quality, response.Code, response.Body.String())
		}
	}
	if response := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads/missing/file", nil); response.Code != http.StatusNotFound {
		t.Fatalf("missing download file = %d %q", response.Code, response.Body.String())
	}
}

// OfflineDownloadRejectsTrailingJSONBeforeCreatingAJob runs the shared download regression contract.
func OfflineDownloadRejectsTrailingJSONBeforeCreatingAJob(t *testing.T, fixture DownloadFixture) {
	handler, token := fixture.APIServer(t)
	itemID := fixture.FirstItemID(t, handler, token)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+itemID+"/downloads", strings.NewReader(`{"quality":"original"}{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON = %d %q", response.Code, response.Body.String())
	}
	if downloads := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads", nil); strings.Contains(downloads.Body.String(), itemID) {
		t.Fatalf("rejected request created a download: %q", downloads.Body.String())
	}
}

func containsJSONID(body, value string) bool { return len(value) > 0 && strings.Contains(body, value) }

func assertOriginalDownloadReady(t *testing.T, fixture DownloadFixture, handler http.Handler, token, jobID string) {
	t.Helper()
	ready := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		status := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads/"+jobID, nil)
		if containsJSONID(status.Body.String(), `"state":"ready"`) {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatal("original download did not become ready")
	}
}
