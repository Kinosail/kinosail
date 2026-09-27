package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPreferredSubtitleLanguageAndPickerLimitAcrossWebAndAPI(t *testing.T) { //nolint:cyclop,funlen // One lifecycle proves selection, filtering, and unchanged files across both adapters.
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	for name, contents := range map[string]string{
		"Arrival.mp4":           "video",
		"Arrival.en.srt":        "1\n00:00:01,000 --> 00:00:02,000\nHello\n",
		"Arrival.fr.forced.srt": "1\n00:00:01,000 --> 00:00:02,000\nForced\n",
		"Arrival.fr.srt":        "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n",
		"Arrival.nl.forced.srt": "1\n00:00:01,000 --> 00:00:02,000\nGedwongen\n",
	} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language=fr"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, request)
	id := firstWebItemID(t, handler)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	apiSettings := httptest.NewRecorder()
	handler.ServeHTTP(apiSettings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/settings", nil))

	if saved.Code != http.StatusSeeOther || !strings.Contains(settings.Body.String(), `name="language" value="fr"`) || !strings.Contains(settings.Body.String(), `href="https://github.com/Kinosail/kinosail/tree/main/apps/subtitles" rel="noreferrer">Kino Subtitles on GitHub</a>`) || strings.Contains(settings.Body.String(), "SubDL") {
		t.Fatalf("save = %d, settings = %q", saved.Code, settings.Body.String())
	}
	if !strings.Contains(player.Body.String(), `<track default kind="subtitles" label="French · Subtitles" data-subtitle-source="`) || !strings.Contains(playback.Body.String(), `"label":"French · Subtitles","source":"/subtitle/`+id+`/2","default":true,"language":"fr"`) {
		t.Fatalf("player = %q, playback = %q", player.Body.String(), playback.Body.String())
	}
	if strings.Contains(apiSettings.Body.String(), "subtitleProvider") {
		t.Fatalf("API settings still expose a subtitle provider: %q", apiSettings.Body.String())
	}
	limitRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/picker", strings.NewReader("limited=on"))
	limitRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	limitSave := httptest.NewRecorder()
	handler.ServeHTTP(limitSave, limitRequest)
	limitedPlayer := httptest.NewRecorder()
	handler.ServeHTTP(limitedPlayer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	limitedPlayback := httptest.NewRecorder()
	handler.ServeHTTP(limitedPlayback, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	if limitSave.Code != http.StatusSeeOther || strings.Contains(limitedPlayer.Body.String(), "English · Subtitles") || strings.Contains(limitedPlayer.Body.String(), "Dutch · Forced") || strings.Contains(limitedPlayback.Body.String(), `"language":"en"`) || strings.Contains(limitedPlayback.Body.String(), `"language":"nl"`) {
		t.Fatalf("unselected track remained visible: save=%d player=%s API=%s", limitSave.Code, limitedPlayer.Body.String(), limitedPlayback.Body.String())
	}
	videoTag := regexp.MustCompile(`<video\b[^>]*>`)
	limitedVideo := videoTag.FindString(limitedPlayer.Body.String())
	if !strings.Contains(limitedVideo, `data-subtitle-picker-limited="true"`) || strings.Contains(limitedVideo, " controls ") {
		t.Fatalf("limited player exposed native subtitle controls: %q", limitedVideo)
	}
	unlimitedVideo := videoTag.FindString(player.Body.String())
	if !strings.Contains(unlimitedVideo, " controls ") {
		t.Fatalf("unlimited player lost native control fallback: %q", unlimitedVideo)
	}
	for _, name := range []string{"Arrival.en.srt", "Arrival.nl.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("subtitle choice filtering changed %s: %v", name, err)
		}
	}
}

func TestRetiredSubtitleRoutes(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
	id := firstWebItemID(t, handler)
	for _, retired := range []struct{ method, path string }{
		{http.MethodPost, "/subtitles/" + id + "/fetch"},
		{http.MethodGet, "/subtitles/" + id + "/fr"},
		{http.MethodPost, "/api/v1/items/" + id + "/subtitles"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), retired.method, retired.path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("retired route %s %s = %d", retired.method, retired.path, response.Code)
		}
	}
}

func TestSubtitlePickerRejectsInvalidValuesWithoutChangingSettings(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	valid := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/picker", strings.NewReader("limited=on"))
	valid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, valid)
	if saved.Code != http.StatusSeeOther {
		t.Fatalf("valid picker setting = %d", saved.Code)
	}
	for _, body := range []string{"", "limited=maybe", "limited=on&limited=off", "limited=on&extra=1", "limited=" + strings.Repeat("x", 4097)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/picker", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid web picker value %q = %d", body, response.Code)
		}
	}
	for _, body := range []string{"{}", "{\"limited\":null}", "{\"limited\":\"on\"}", "{\"limited\":true,\"extra\":1}", "{\"limited\":true,\"limited\":false}"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/subtitle-picker", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid API picker value %q = %d", body[:min(len(body), 50)], response.Code)
		}
	}
	oversized := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/subtitle-picker", strings.NewReader("{\"limited\":"+strings.Repeat(" ", 1<<20)+"false}"))
	oversized.Header.Set("Content-Type", "application/json")
	oversizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(oversizedResponse, oversized)
	if oversizedResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized API picker input = %d", oversizedResponse.Code)
	}
	stillLimited := httptest.NewRecorder()
	handler.ServeHTTP(stillLimited, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/settings", nil))
	if !strings.Contains(stillLimited.Body.String(), `"subtitlePickerLimited":true`) {
		t.Fatal("rejected picker input changed saved choices")
	}
}

func TestPreferredSubtitleLanguageRejectsInvalidValuesWithoutChangingSettings(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	for _, value := range []string{"", "z", "zz", "english", "e1"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language="+value))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid language %q = %d", value, response.Code)
		}
	}
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if match := regexp.MustCompile(`name="language" value="([^"]+)"`).FindStringSubmatch(settings.Body.String()); len(match) != 2 || match[1] != "en" {
		t.Fatalf("subtitle language changed after rejected input: %q", settings.Body.String())
	}
}
