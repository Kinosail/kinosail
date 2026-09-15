package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestKinosailPlayerDetectsNewSubtitleWithoutRestart(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			writeSubDLSearch(writer, request.Host, "/arrival.srt", "Arrival")
		case "/arrival.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nDetected\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	player := server.New(server.Config{Lifecycle: lifecycle, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	subtitles := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	id := firstSubtitleInventoryID(t, subtitles)
	response := requestJSON(t, subtitles, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("subtitle write = %d %q", response.Code, response.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		page := requestApp(t, player, http.MethodGet, "/watch/"+id, "")
		if page.Code == http.StatusOK && strings.Contains(page.Body.String(), `label="EN"`) {
			cancel()
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("Kinosail Player did not detect the new subtitle")
}
