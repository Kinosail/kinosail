package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func (fixture LibraryAPIFixture) LegacySubtitleFilesAreServedAsUTF8(t *testing.T) {
	mediaDir := t.TempDir()
	files := map[string]string{
		"Film.mp4":    "video",
		"Film.fr.srt": "1\r\n00:00:00,000 --> 00:00:01,000\r\nCaf\xe9 cr\xe8me\r\n",
		"Film.ru.srt": "1\n00:00:00,000 --> 00:00:01,000\n\xcf\xf0\xe8\xe2\xe5\xf2\n",
		"Film.de.vtt": "\xff\xfeW\x00E\x00B\x00V\x00T\x00T\x00\n\x00\n\x000\x000\x00:\x000\x000\x00.\x000\x000\x000\x00 \x00-\x00-\x00>\x00 \x000\x000\x00:\x000\x001\x00.\x000\x000\x000\x00\n\x00G\x00r\x00\xfc\x00\xdf\x00e\x00\n\x00",
		"Film.en.srt": "\xef\xbb\xbf1\n00:00:00,000 --> 00:00:01,000\nHello\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	var texts []string
	for track := range 4 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/"+id+"/"+strconv.Itoa(track), nil))
		body := response.Body.String()
		if response.Code != http.StatusOK || !utf8.ValidString(body) || !strings.HasPrefix(body, "WEBVTT") || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/vtt") {
			t.Fatalf("track %d = %d %q %q", track, response.Code, response.Header().Get("Content-Type"), body)
		}
		lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n")), "\n")
		texts = append(texts, lines[len(lines)-1])
	}
	slices.Sort(texts)
	if want := []string{"Café crème", "Grüße", "Hello", "Привет"}; !slices.Equal(texts, want) {
		t.Fatalf("subtitle text = %q, want %q", texts, want)
	}
}
