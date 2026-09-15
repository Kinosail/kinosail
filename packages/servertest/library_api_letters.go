package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type libraryLetterBucket struct {
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Offset int    `json:"offset"`
}

func (fixture LibraryAPIFixture) LetterJumpIsSharedByAPIAndWeb(t *testing.T) {
	mediaDir := t.TempDir()
	for _, name := range []string{"Alpha.mp4", "Beta.mp4", "Gamma.mp4", "Grinch.mp4", "Home.mp4", "Жизнь.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&limit=2&letter=g", nil)
	assertLibraryLetterPage(t, response)
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&limit=2&letter=G", nil))
	body := web.Body.String()
	if web.Code != http.StatusOK || !strings.Contains(body, `aria-label="Jump to title"`) || !strings.Contains(body, `aria-current="true">G</a>`) || !strings.Contains(body, ">Gamma</h2>") || !strings.Contains(body, ">Grinch</h2>") {
		t.Fatalf("letter web = %d %q", web.Code, body)
	}
}

func assertLibraryLetterPage(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var page struct {
		Items   []struct{ Title string } `json:"items"`
		Letter  string                   `json:"letter"`
		Offset  int                      `json:"offset"`
		Letters []libraryLetterBucket    `json:"letters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || page.Letter != "G" || page.Offset != 2 || len(page.Items) != 2 || page.Items[0].Title != "Gamma" || page.Items[1].Title != "Grinch" {
		t.Fatalf("letter API = %#v, status = %d, error = %v, body = %q", page, response.Code, err, response.Body.String())
	}
	assertCyrillicLibraryBucket(t, page.Letters)
}

func assertCyrillicLibraryBucket(t *testing.T, letters []libraryLetterBucket) {
	t.Helper()
	foundCyrillic := false
	for _, bucket := range letters {
		if bucket.Label == "Ж" && bucket.Count == 1 {
			foundCyrillic = true
		}
	}
	if !foundCyrillic {
		t.Fatalf("letter buckets = %#v", letters)
	}
}
