package downloads

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkOriginalDownloadPreparation(b *testing.B) {
	root := b.TempDir()
	source := filepath.Join(root, "source.mkv")
	data := make([]byte, 64<<20)
	if _, err := rand.NewChaCha8([32]byte{}).Read(data); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0o600); err != nil {
		b.Fatal(err)
	}
	output := filepath.Join(root, "download.mkv")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		manifest, err := copyManifest(b.Context(), source, output, "0123456789abcdef")
		if err != nil || manifest.Size != int64(len(data)) {
			b.Fatalf("manifest = %#v, %v", manifest, err)
		}
	}
}
