package auditjournal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkMediaRangeDelivery(b *testing.B) {
	path := filepath.Join(b.TempDir(), "media.mp4")
	if err := os.WriteFile(path, make([]byte, 8<<20), 0o600); err != nil {
		b.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(&ResponseWriter{ResponseWriter: writer}, request, path)
	}))
	b.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 32}}
	b.Cleanup(client.CloseIdleConnections)
	b.SetBytes(4 << 20)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request, err := http.NewRequestWithContext(b.Context(), http.MethodGet, server.URL, nil)
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
