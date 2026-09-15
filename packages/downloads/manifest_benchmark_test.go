package downloads

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Run only after quality gates are explicitly enabled. This isolates response
// startup from network throughput; physical-device goodput remains a separate test.
func BenchmarkSealedDownloadHEAD(b *testing.B) {
	path := filepath.Join(b.TempDir(), "media.mp4")
	file, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	if err = file.Truncate(64 << 20); err != nil {
		b.Fatal(err)
	}
	if err = file.Close(); err != nil {
		b.Fatal(err)
	}
	manifest, err := sealManifest(path, "0123456789abcdef")
	if err != nil {
		b.Fatal(err)
	}
	job := Job{ID: manifest.ID, File: path, Title: "Benchmark", Size: manifest.Size, SHA256: manifest.SHA256}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		request := httptest.NewRequestWithContext(b.Context(), http.MethodHead, "/file", nil)
		response := httptest.NewRecorder()
		if err := Serve(response, request, job); err != nil {
			b.Fatal(err)
		}
		if response.Body.Len() != 0 {
			b.Fatal("HEAD returned media")
		}
	}
}
