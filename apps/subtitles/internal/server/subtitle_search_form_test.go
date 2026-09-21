package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestSubtitleSearchSubmitsRenderedFormWithoutCSRFQuery(t *testing.T) {
	media := t.TempDir()
	for name, content := range map[string]string{"Covered Film.mp4": "video", "Covered Film.en.srt": "1\n00:00:01,000 --> 00:00:02,000\nHello\n"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	cookie := &http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	for _, view := range []string{"summary", "wanted", "library"} {
		t.Run(view, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view="+view, nil)
			request.AddCookie(cookie)
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, request)
			query := subtitleSearchFormValues(t, page.Body.String())
			query.Set("q", "Covered Film")
			if query.Has("_csrf") {
				t.Fatal("search form submits a CSRF query field")
			}
			expected := "library"
			if view == "wanted" {
				expected = "wanted"
			}
			if query.Get("view") != expected {
				t.Fatalf("search scope=%q, want %q", query.Get("view"), expected)
			}
			searchRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?"+query.Encode(), nil)
			searchRequest.AddCookie(cookie)
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, searchRequest)
			body := result.Body.String()
			if result.Code != http.StatusOK || !strings.Contains(body, `>Clear filters</a>`) {
				t.Fatalf("search failed: %d %s", result.Code, body)
			}
			found := strings.Contains(body, `>Covered Film</strong>`)
			if found != (view != "wanted") {
				t.Fatalf("covered film visibility=%v for %s", found, view)
			}
		})
	}
}

func subtitleSearchFormValues(t *testing.T, body string) url.Values {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for node := range root.Descendants() {
		if node.Type == html.ElementNode && node.Data == "form" && subtitleHTMLAttribute(node, "role") == "search" {
			return subtitleFormInputs(node)
		}
	}
	t.Fatal("search form missing")
	return nil
}

func subtitleFormInputs(form *html.Node) url.Values {
	result := url.Values{}
	for node := range form.Descendants() {
		if node.Type == html.ElementNode && node.Data == "input" {
			result.Add(subtitleHTMLAttribute(node, "name"), subtitleHTMLAttribute(node, "value"))
		}
	}
	return result
}

func subtitleHTMLAttribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}
