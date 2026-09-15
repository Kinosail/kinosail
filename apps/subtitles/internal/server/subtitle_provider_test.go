package server_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerCanFetchProviderSubtitle(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(fakeSubDL))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.BluRay-GROUP.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	fetch := httptest.NewRecorder()
	handler.ServeHTTP(fetch, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	track := httptest.NewRecorder()
	handler.ServeHTTP(track, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitles/"+id+"/en", nil))

	if fetch.Code != http.StatusSeeOther || !strings.Contains(player.Body.String(), `label="EN · Provider"`) || track.Header().Get("Content-Type") != "text/vtt; charset=utf-8" || !strings.Contains(track.Body.String(), "WEBVTT") {
		t.Fatalf("fetch = %d %q, player = %q, track = %d %q", fetch.Code, fetch.Body.String(), player.Body.String(), track.Code, track.Body.String())
	}
}

func TestSubtitleAutomationBoundsEachProviderCycle(t *testing.T) {
	t.Parallel()
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			writeSubDLSearch(writer, request.Host, "/subtitle.srt", request.URL.Query().Get("film_name"))
		case "/subtitle.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nAutomatic\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media, data := t.TempDir(), t.TempDir()
	for number := 1; number <= 12; number++ {
		name := filepath.Join(media, fmt.Sprintf("Film %02d.BluRay-GROUP.mp4", number))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	health := `{"version":1,"providers":{"SubDL":{"last_success":` + strconv.FormatInt(time.Now().Unix(), 10) + `}}}`
	writeTestFile(t, filepath.Join(data, "subtitle_provider_health.json"), health)
	_ = server.New(server.Config{Lifecycle: lifecycle, SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), BackupDir: t.TempDir(), BackupKey: "test-encryption-key", Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		written, _ := filepath.Glob(filepath.Join(media, "*.en.srt"))
		if len(written) == 10 {
			time.Sleep(250 * time.Millisecond)
			written, _ = filepath.Glob(filepath.Join(media, "*.en.srt"))
			if len(written) != 10 {
				t.Fatalf("automatic cycle wrote %d sidecars", len(written))
			}
			cancel()
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("automatic provider cycle did not write ten sidecars")
}

func fakeSubDL(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // One fixture emulates every bounded SubDL response used by tests.
	if request.URL.Query().Get("api_key") != "key" && request.URL.Path != "/arrival.zip" {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch request.URL.Path {
	case "/subtitles":
		if request.URL.Query().Get("film_name") != "Arrival" || request.URL.Query().Get("file_name") != "Arrival.BluRay-GROUP.mp4" || request.URL.Query().Get("languages") != "EN" || request.URL.Query().Get("type") != "movie" || request.URL.Query().Get("releases") != "1" || request.URL.Query().Get("unpack") != "1" {
			http.Error(writer, "bad query", http.StatusBadRequest)
			return
		}
		writeSubDLSearch(writer, request.Host, "/arrival.zip", "Arrival")
	case "/arrival.zip":
		var archive bytes.Buffer
		files := zip.NewWriter(&archive)
		file, _ := files.Create("Arrival.BluRay-GROUP.en.srt")
		_, _ = file.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
		_ = files.Close()
		_, _ = writer.Write(archive.Bytes())
	default:
		http.NotFound(writer, request)
	}
}

func writeSubDLSearch(writer http.ResponseWriter, host, path, title string) {
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"status":  true,
		"results": []any{map[string]any{"name": title, "type": "movie"}},
		"subtitles": []any{map[string]any{
			"url":          "http://" + host + path,
			"language":     "EN",
			"release_name": title + ".BluRay-GROUP",
		}},
	})
}

func TestSubtitleProviderFailureDoesNotBlockPlayback(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.BluRay-GROUP.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	fetch := httptest.NewRecorder()
	handler.ServeHTTP(fetch, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if fetch.Code != http.StatusBadGateway || player.Code != http.StatusOK || !strings.Contains(player.Body.String(), "Arrival") {
		t.Fatalf("fetch = %d, player = %d %q", fetch.Code, player.Code, player.Body.String())
	}
}

func TestSubtitleProviderRejectsInvalidResponsesBeforeDownload(t *testing.T) {
	t.Parallel()
	for name, response := range map[string]func(string) string{
		"trailing": func(host string) string {
			return `{"status":true,"subtitles":[{"url":"http://` + host + `/download"}]}{}`
		},
		"too many": func(host string) string {
			items := make([]map[string]string, 31)
			for index := range items {
				items[index] = map[string]string{"url": "http://" + host + "/download"}
			}
			data, _ := json.Marshal(map[string]any{"status": true, "subtitles": items})
			return string(data)
		},
	} {
		t.Run(name, func(t *testing.T) {
			downloads := 0
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/download" {
					downloads++
					_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nWrong\n"))
					return
				}
				_, _ = writer.Write([]byte(response(request.Host)))
			}))
			t.Cleanup(provider.Close)
			media, cache := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			handler := server.New(server.Config{MediaDir: media, CacheDir: cache, Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
			home := httptest.NewRecorder()
			handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
			fetch := httptest.NewRecorder()
			handler.ServeHTTP(fetch, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", nil))
			if fetch.Code != http.StatusBadGateway || downloads != 0 {
				t.Fatalf("invalid subtitle response = %d, downloads = %d", fetch.Code, downloads)
			}
		})
	}
}

func TestSubtitleProviderRejectsInvalidEndpointWithoutPanic(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: "://invalid", APIKey: "key"}})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	fetch := httptest.NewRecorder()
	handler.ServeHTTP(fetch, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", nil))
	if fetch.Code != http.StatusBadGateway {
		t.Fatalf("invalid subtitle endpoint = %d %q", fetch.Code, fetch.Body.String())
	}
}

func TestOwnerCanChooseSubtitleLanguage(t *testing.T) { //nolint:cyclop // One settings scenario covers selection and provider behavior.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.BluRay-GROUP.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{APIKey: "key"}})
	for _, body := range []string{"preference=sdh", "language=es&preference=unknown", "language=es&preference=sdh&preference=standard", "language=es&preference=sdh&unknown=x", "language=" + strings.Repeat("x", 4097)} {
		invalid := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader(body))
		invalid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		invalidResponse := httptest.NewRecorder()
		handler.ServeHTTP(invalidResponse, invalid)
		if invalidResponse.Code != http.StatusBadRequest {
			t.Fatalf("invalid subtitle plan %q = %d", body, invalidResponse.Code)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language=es&preference=sdh"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	configuration := httptest.NewRecorder()
	handler.ServeHTTP(configuration, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/configuration", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	subtitleSettings := httptest.NewRecorder()
	subtitleHandler := server.New(server.Config{SubtitleApp: true, MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{APIKey: "key"}})
	subtitleHandler.ServeHTTP(subtitleSettings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(subtitleSettings.Body.String(), `name="preference" value="sdh" checked`) || !strings.Contains(settings.Body.String(), `name="language" value="es"`) || !strings.Contains(settings.Body.String(), "Embedded first · SubDL · OpenSubtitles · SubSource") || !strings.Contains(settings.Body.String(), `href="/settings/configuration#integrations.subdl.api_key"`) || !strings.Contains(settings.Body.String(), "Credentials configured") || strings.Contains(settings.Body.String(), `value="key"`) || !strings.Contains(configuration.Body.String(), `id="integrations.subdl.api_key"`) || !strings.Contains(configuration.Body.String(), `id="integrations.opensubtitles"`) || !strings.Contains(player.Body.String(), `<span>Subtitles</span><strong>Find es</strong>`) {
		t.Fatalf("save = %d, settings = %q, configuration = %q, player = %q", response.Code, settings.Body.String(), configuration.Body.String(), player.Body.String())
	}
}
