package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestConcurrentSubtitleFetchNeverOverwritesSidecar(t *testing.T) { //nolint:cyclop // The race contract needs both concurrent responses and the final file state.
	t.Parallel()
	var searches atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			if searches.Add(1) == 2 {
				once.Do(func() { close(release) })
			}
			select {
			case <-release:
			case <-time.After(2 * time.Second):
				http.Error(writer, "concurrent request did not arrive", http.StatusInternalServerError)
				return
			}
			writeSubDLSearch(writer, request.Host, "/subtitle.srt", "Arrival")
		case "/subtitle.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nConcurrent\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	home := requestApp(t, handler, http.MethodGet, "/", "")
	id := regexp.MustCompile(`/subtitles/manage/([a-f0-9]+)/fetch`).FindStringSubmatch(home.Body.String())[1]
	start, statuses := make(chan struct{}), make(chan int, 2)
	for range 2 {
		go func() {
			<-start
			statuses <- requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{}`).Code
		}()
	}
	close(start)
	counts := map[int]int{<-statuses: 1}
	counts[<-statuses]++
	if counts[http.StatusCreated] != 1 || counts[http.StatusConflict] != 1 || searches.Load() != 2 {
		t.Fatalf("statuses = %v, searches = %d", counts, searches.Load())
	}
	if data, err := os.ReadFile(filepath.Join(media, "Arrival.BluRay-GROUP.en.srt")); err != nil || !strings.Contains(string(data), "Concurrent") {
		t.Fatalf("sidecar = %q, %v", data, err)
	}
}
