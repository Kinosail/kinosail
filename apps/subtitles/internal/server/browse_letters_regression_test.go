package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLetterJumpBucketsStayUniqueWithPunctuatedTitles(t *testing.T) {
	mediaDir := t.TempDir()
	for _, name := range []string{"'Cruella.mp4", "'Twas.mp4", "2001.mp4", "Alpha.mp4", "Beta.mp4", "Cruella.mp4", "Movie.mp4", "Title.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(Config{MediaDir: mediaDir})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library?view=movies&letter=M", nil))
	var page struct {
		Letters []struct{ Label string } `json:"letters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK {
		t.Fatalf("letter API = %d, error = %v, body = %q", response.Code, err, response.Body.String())
	}
	var labels []string
	for _, bucket := range page.Letters {
		labels = append(labels, bucket.Label)
	}
	if strings.Join(labels, "") != "#ABCMT" {
		t.Fatalf("letter buckets = %q", labels)
	}

}
