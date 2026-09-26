package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleSettingsCleanupJourney(t *testing.T) { //nolint:cyclop // The public journey verifies preview, deletion, preferences, and preserved files together.
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt", "Film.es.srt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	if response := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es"]}`); response.Code != http.StatusOK {
		t.Fatalf("set languages: %d %s", response.Code, response.Body.String())
	}
	settings := requestApp(t, handler, http.MethodGet, "/settings", "")
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), "Delete subtitle languages") || !strings.Contains(settings.Body.String(), `name="forced"`) {
		t.Fatalf("cleanup setting: %d %s", settings.Code, settings.Body.String())
	}
	preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?enabled=on&language=en&forced=keep", "")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "1 subtitle file to delete") || !strings.Contains(preview.Body.String(), "Film.es.srt") || strings.Contains(preview.Body.String(), "<li><code>"+filepath.Join(media, "Film.en.forced.srt")) {
		t.Fatalf("cleanup preview: %d %s", preview.Code, preview.Body.String())
	}
	match := regexp.MustCompile(`name="digest" value="([0-9a-f]{64})"`).FindStringSubmatch(preview.Body.String())
	if len(match) != 2 {
		t.Fatal("preview digest missing")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader("enabled=on&language=en&forced=keep&digest="+match[1]))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Deleted 1 subtitle file") {
		t.Fatalf("delete: %d %s", result.Code, result.Body.String())
	}
	if _, err := os.Stat(filepath.Join(media, "Film.es.srt")); !os.IsNotExist(err) {
		t.Fatalf("Spanish subtitle remains: %v", err)
	}
	if settings := requestApp(t, handler, http.MethodGet, "/api/v1/settings", ""); !strings.Contains(settings.Body.String(), `"subtitleLanguages":["en"]`) {
		t.Fatalf("cleanup preferences: %d %s", settings.Code, settings.Body.String())
	}
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("kept subtitle %s: %v", name, err)
		}
	}
}

func TestSubtitleCleanupAPIRequiresOptInAndKeepsSelectedLanguages(t *testing.T) {
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	for _, name := range []string{"Film.en.srt", "Film.es.srt", "Film.fr.srt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	path := "/api/v1/subtitles/cleanup"
	for _, body := range []string{
		`{"languages":["en","es"],"forced":"keep"}`,
		`{"enabled":false,"languages":["en","es"],"forced":"keep"}`,
		`{"enabled":true,"languages":["en","es"],"forced":"keep","unknown":true}`,
		`{"enabled":true,"languages":[],"forced":"keep"}`,
		`{"enabled":true,"languages":["en","en"],"forced":"keep"}`,
		`{"enabled":true,"languages":["en","invalid"],"forced":"keep"}`,
		strings.Repeat(" ", 4097),
	} {
		response := requestJSON(t, handler, http.MethodPost, path+"/preview", body)
		if response.Code != http.StatusBadRequest {
			t.Errorf("accepted invalid API preview %s: %d", body, response.Code)
		}
	}
	if response := requestJSON(t, handler, http.MethodPost, path+"/preview?extra=1", `{"enabled":true,"languages":["en"],"forced":"keep"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("query accepted: %d %s", response.Code, response.Body.String())
	}
	wrongType := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+"/preview", strings.NewReader(`{"enabled":true,"languages":["en"],"forced":"keep"}`))
	wrongType.Header.Set("Content-Type", "text/plain")
	wrongResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongResponse, wrongType)
	if wrongResponse.Code != http.StatusBadRequest {
		t.Fatalf("content type accepted: %d %s", wrongResponse.Code, wrongResponse.Body.String())
	}
	preview := requestJSON(t, handler, http.MethodPost, path+"/preview", `{"enabled":true,"languages":["en","es"],"forced":"keep"}`)
	if preview.Code != http.StatusOK {
		t.Fatalf("API preview: %d %s", preview.Code, preview.Body.String())
	}
	var plan struct {
		Digest string `json:"digest"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &plan); err != nil || plan.Count != 1 || len(plan.Digest) != 64 {
		t.Fatalf("API preview plan: %+v %v", plan, err)
	}
	for _, body := range []string{
		`{"enabled":false,"languages":["en","es"],"forced":"keep","digest":"` + plan.Digest + `"}`,
		`{"enabled":true,"languages":["es","en"],"forced":"keep","digest":"` + plan.Digest + `"}`,
	} {
		response := requestJSON(t, handler, http.MethodPost, path, body)
		if response.Code < http.StatusBadRequest {
			t.Errorf("accepted invalid API confirmation %s: %d", body, response.Code)
		}
		if _, err := os.Stat(filepath.Join(media, "Film.fr.srt")); err != nil {
			t.Fatalf("invalid API request removed subtitle: %v", err)
		}
	}
	result := requestJSON(t, handler, http.MethodPost, path, `{"enabled":true,"languages":["en","es"],"forced":"keep","digest":"`+plan.Digest+`"}`)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"removed":1`) {
		t.Fatalf("API confirmation: %d %s", result.Code, result.Body.String())
	}
	if _, err := os.Stat(filepath.Join(media, "Film.fr.srt")); !os.IsNotExist(err) {
		t.Fatalf("French subtitle remains: %v", err)
	}
}

func TestSubtitleCleanupCannotOverrideManagedLanguage(t *testing.T) {
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	other := filepath.Join(media, "Film.es.srt")
	writeTestFile(t, other, "Spanish subtitles")
	configured, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_SUBTITLE_LANGUAGE" {
			return "es", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Configuration: configured})
	preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?enabled=on&language=en&forced=keep", "")
	if preview.Code != http.StatusConflict {
		t.Fatalf("managed preview: %d %s", preview.Code, preview.Body.String())
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("managed cleanup changed media: %v", err)
	}
}

func TestSubtitleCleanupRequiresOptInAndKeepsSeveralLanguages(t *testing.T) {
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt", "Film.es.srt", "Film.es.forced.srt", "Film.fr.srt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	if response := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es","fr"]}`); response.Code != http.StatusOK {
		t.Fatalf("set languages: %d %s", response.Code, response.Body.String())
	}
	settings := requestApp(t, handler, http.MethodGet, "/settings", "")
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), `name="enabled" value="on"`) || strings.Contains(settings.Body.String(), `name="enabled" value="on" checked`) {
		t.Fatalf("cleanup must start off: %d %s", settings.Code, settings.Body.String())
	}
	for _, query := range []string{
		"language=en&language=es&forced=keep",
		"enabled=off&language=en&language=es&forced=keep",
		"enabled=on&forced=keep",
		"enabled=on&language=en&language=en&forced=keep",
		"enabled=on&language=en&language=invalid&forced=keep",
		"enabled=on&language=en&language=en-US&forced=keep",
		"enabled=on&" + strings.Repeat("language=en&", 21) + "forced=keep",
		"enabled=on&language=" + strings.Repeat("x", 2050) + "&forced=keep",
	} {
		preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?"+query, "")
		if preview.Code != http.StatusBadRequest {
			t.Errorf("accepted invalid preview %q: %d", query, preview.Code)
		}
	}
	preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?enabled=on&language=en&language=es&forced=keep", "")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "1 subtitle file to delete") || !strings.Contains(preview.Body.String(), "Film.fr.srt") || strings.Contains(preview.Body.String(), "<li><code>"+filepath.Join(media, "Film.es.srt")) {
		t.Fatalf("multi-language preview: %d %s", preview.Code, preview.Body.String())
	}
	match := regexp.MustCompile(`name="digest" value="([0-9a-f]{64})"`).FindStringSubmatch(preview.Body.String())
	if len(match) != 2 {
		t.Fatal("preview digest missing")
	}
	postCleanup := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for _, body := range []string{
		"language=en&language=es&forced=keep&digest=" + match[1],
		"enabled=on&language=en&language=es&language=es&forced=keep&digest=" + match[1],
		"enabled=on&language=es&language=en&forced=keep&digest=" + match[1],
	} {
		result := postCleanup(body)
		if result.Code == http.StatusOK {
			t.Errorf("accepted invalid confirmation %q", body)
		}
		if _, err := os.Stat(filepath.Join(media, "Film.fr.srt")); err != nil {
			t.Fatalf("invalid request removed subtitle: %v", err)
		}
	}
	result := postCleanup("enabled=on&language=en&language=es&forced=keep&digest=" + match[1])
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Deleted 1 subtitle file") {
		t.Fatalf("confirmation: %d %s", result.Code, result.Body.String())
	}
	if _, err := os.Stat(filepath.Join(media, "Film.fr.srt")); !os.IsNotExist(err) {
		t.Fatalf("French subtitle remains: %v", err)
	}
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt", "Film.es.srt", "Film.es.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("kept subtitle %s: %v", name, err)
		}
	}
	if settings := requestApp(t, handler, http.MethodGet, "/api/v1/settings", ""); !strings.Contains(settings.Body.String(), `"subtitleLanguages":["en","es"]`) {
		t.Fatalf("kept preferences: %d %s", settings.Code, settings.Body.String())
	}
	forcedPreview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?enabled=on&language=en&language=es&forced=delete", "")
	if forcedPreview.Code != http.StatusOK || !strings.Contains(forcedPreview.Body.String(), "2 subtitle files to delete") {
		t.Fatalf("forced preview: %d %s", forcedPreview.Code, forcedPreview.Body.String())
	}
	forcedMatch := regexp.MustCompile(`name="digest" value="([0-9a-f]{64})"`).FindStringSubmatch(forcedPreview.Body.String())
	if len(forcedMatch) != 2 {
		t.Fatal("forced preview digest missing")
	}
	forcedResult := postCleanup("enabled=on&language=en&language=es&forced=delete&digest=" + forcedMatch[1])
	if forcedResult.Code != http.StatusOK || !strings.Contains(forcedResult.Body.String(), "Deleted 2 subtitle files") {
		t.Fatalf("forced confirmation: %d %s", forcedResult.Code, forcedResult.Body.String())
	}
	for _, name := range []string{"Film.en.forced.srt", "Film.es.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); !os.IsNotExist(err) {
			t.Fatalf("forced subtitle remains %s: %v", name, err)
		}
	}
}
