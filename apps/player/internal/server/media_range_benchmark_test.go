package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func BenchmarkMediaRangeThroughServer(b *testing.B) { //nolint:cyclop,gocognit // Keep fixture discovery and full range validation in the measured server benchmark.
	media := b.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Stream.mp4"), make([]byte, 8<<20), 0o600); err != nil {
		b.Fatal(err)
	}
	instance := httptest.NewServer(server.New(server.Config{MediaDir: media, DataDir: b.TempDir()}))
	b.Cleanup(instance.Close)
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 32}}
	b.Cleanup(client.CloseIdleConnections)
	homeRequest, err := http.NewRequestWithContext(b.Context(), http.MethodGet, instance.URL, nil)
	if err != nil {
		b.Fatal(err)
	}
	home, err := client.Do(homeRequest)
	if err != nil {
		b.Fatal(err)
	}
	page, err := io.ReadAll(home.Body)
	if closeErr := home.Body.Close(); err != nil || closeErr != nil {
		b.Fatalf("home: read=%v close=%v", err, closeErr)
	}
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindSubmatch(page)
	if len(id) != 2 {
		b.Fatal("fixture has no media ID")
	}
	url := instance.URL + "/media/" + string(id[1])
	b.SetBytes(4 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request, err := http.NewRequestWithContext(b.Context(), http.MethodGet, url, nil)
			if err != nil {
				b.Fatal(err)
			}
			request.Header.Set("Range", "bytes=0-4194303")
			response, err := client.Do(request)
			if err != nil {
				b.Fatal(err)
			}
			read, err := io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			if err != nil || closeErr != nil || response.StatusCode != http.StatusPartialContent || read != 4<<20 {
				b.Fatalf("range: status=%d bytes=%d read=%v close=%v", response.StatusCode, read, err, closeErr)
			}
		}
	})
}
