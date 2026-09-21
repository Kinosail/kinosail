package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JellyfinDownloadFixture supplies authenticated, trusted compatibility handlers.
type JellyfinDownloadFixture struct {
	New    func(*testing.T, string, string, string, bool) http.Handler
	SignIn func(*testing.T, http.Handler, string, string) *http.Cookie
	Login  func(*testing.T, http.Handler, *http.Cookie) (string, string)
	Call   func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
}

func (fixture JellyfinDownloadFixture) ResumesConditionallyAcrossReplacementAndRestart(t *testing.T) { //nolint:cyclop // One compatibility test covers conditional resumes across file replacement and restart.
	t.Parallel()
	media, data, cache := t.TempDir(), t.TempDir(), t.TempDir()
	path := filepath.Join(media, "Film.mp4")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(t, media, data, cache, false)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := fixture.Login(t, handler, owner)
	items := fixture.Call(t, handler, http.MethodGet, "/Items", "", token)
	var catalog struct{ Items []struct{ ID string } }
	MustJSON(t, items, &catalog)
	downloadURL := "/Items/" + catalog.Items[0].ID + "/Download"
	original := fixture.Call(t, handler, http.MethodGet, downloadURL, "", token)
	if original.Code != http.StatusOK || original.Body.String() != "0123456789" || !strings.HasPrefix(original.Header().Get("ETag"), `"`) || !strings.HasPrefix(original.Header().Get("Repr-Digest"), "sha-256=:") {
		t.Fatalf("original = %d %q %v", original.Code, original.Body.String(), original.Header())
	}
	if err := os.WriteFile(path, []byte("abcdefghij"), 0o600); err != nil {
		t.Fatal(err)
	}
	resumed := jellyfinRange(t, handler, downloadURL, token, "bytes=5-", original.Header().Get("ETag"))
	if resumed.Code != http.StatusOK || resumed.Body.String() != "abcdefghij" || resumed.Header().Get("ETag") == original.Header().Get("ETag") {
		t.Fatalf("resume after replacement = %d %q %v", resumed.Code, resumed.Body.String(), resumed.Header())
	}
	replacementETag := resumed.Header().Get("ETag")
	handler = fixture.New(t, media, data, cache, true)
	resumed = jellyfinRange(t, handler, downloadURL, token, "bytes=5-", replacementETag)
	if resumed.Code != http.StatusPartialContent || resumed.Body.String() != "fghij" || resumed.Header().Get("ETag") != replacementETag {
		t.Fatalf("resume after restart = %d %q %v", resumed.Code, resumed.Body.String(), resumed.Header())
	}
	complete := jellyfinRange(t, handler, downloadURL, token, "bytes=10-", "")
	if complete.Code != http.StatusRequestedRangeNotSatisfiable || complete.Body.Len() != 0 || complete.Header().Get("Content-Range") != "bytes */10" {
		t.Fatalf("complete range = %d %q %v", complete.Code, complete.Body.String(), complete.Header())
	}
}

func jellyfinRange(t *testing.T, handler http.Handler, path, token, value, ifRange string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Jellyfin Android", Device="Phone", DeviceId="phone-1", Version="3", Token="`+token+`"`)
	request.Header.Set("Range", value)
	request.Header.Set("If-Range", ifRange)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
