package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLetterJumpBucketsStayUniqueWithPunctuatedTitles(t *testing.T) { //nolint:cyclop // One fixture compares exact API and web projections.
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

	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&letter=M", nil))
	body := web.Body.String()
	toolbar := strings.Index(body, `<form class="browse-toolbar"`)
	start := strings.Index(body, `<nav class="letter-jump"`)
	if toolbar < 0 || start < toolbar {
		t.Fatalf("title jump must follow browse controls for thumb reach: toolbar = %d, jump = %d", toolbar, start)
	}
	if start < 0 {
		t.Fatalf("letter jump missing from web response = %d %q", web.Code, body)
	}
	end := strings.Index(body[start:], `</nav>`)
	if end < 0 {
		t.Fatalf("letter jump missing from web response = %d %q", web.Code, body)
	}
	matches := regexp.MustCompile(`data-title-letter="([^"]+)"`).FindAllStringSubmatch(body[start:start+end], -1)
	rendered := make([]string, 0, len(matches))
	for _, match := range matches {
		rendered = append(rendered, match[1])
	}
	if web.Code != http.StatusOK || strings.Join(rendered, "") != "#ABCMT" || !strings.Contains(body[start:start+end], `href="/?view=movies" data-title-letter="M"`) || !strings.Contains(body[start:start+end], `aria-label="M, 1 title, selected; activate to show all titles"`) || !strings.Contains(body[start:start+end], `aria-current="true">M</a>`) {
		t.Fatalf("web letter buckets = %q, status = %d", rendered, web.Code)
	}
}
