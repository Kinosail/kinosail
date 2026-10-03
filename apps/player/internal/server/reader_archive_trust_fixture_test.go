package server_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type readerTrustEntry struct {
	name         string
	body         []byte
	declaredSize uint64
}

func readerTrustArchive(t *testing.T, format string, entries []readerTrustEntry) []byte { //nolint:cyclop,gocognit // Archive fixtures retain ordered ZIP and TAR creation with per-step failures.
	t.Helper()
	var archive bytes.Buffer
	if format == "cbt" {
		if _, err := exec.LookPath("bsdtar"); err != nil {
			t.Skip("CBT Reader requires configured bsdtar")
		}
		writer := tar.NewWriter(&archive)
		for _, entry := range entries {
			if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o600, Size: int64(len(entry.body))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return archive.Bytes()
	}
	writer := zip.NewWriter(&archive)
	for _, entry := range entries {
		var resource io.Writer
		var err error
		if entry.declaredSize == 0 {
			resource, err = writer.Create(entry.name)
		} else {
			resource, err = writer.CreateRaw(&zip.FileHeader{Name: entry.name, Method: zip.Store, CRC32: crc32.ChecksumIEEE(entry.body), CompressedSize64: uint64(len(entry.body)), UncompressedSize64: entry.declaredSize})
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := resource.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func readerTrustEPUB(manifest, spine string, chapters ...readerTrustEntry) []readerTrustEntry {
	return append([]readerTrustEntry{
		{name: "META-INF/container.xml", body: []byte(`<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", body: []byte("<package><manifest>" + manifest + "</manifest><spine>" + spine + "</spine></package>")},
	}, chapters...)
}

func readerTrustFixture(t *testing.T, format string, entries []readerTrustEntry) (http.Handler, string, []byte) {
	t.Helper()
	root := t.TempDir()
	media := filepath.Join(root, "media")
	if err := os.Mkdir(media, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := readerTrustArchive(t, format, entries)
	archivePath := filepath.Join(media, "Archive."+format)
	before := sha256.Sum256(archive)
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("private outside-media archive sentinel")
	sentinelPath := filepath.Join(root, "outside.xhtml")
	if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := os.ReadFile(archivePath)
		if err != nil || sha256.Sum256(after) != before {
			t.Errorf("source archive changed: %v", err)
		}
		outside, err := os.ReadFile(sentinelPath)
		if err != nil || !bytes.Equal(outside, sentinel) {
			t.Errorf("outside-media sentinel changed: %v", err)
		}
	})
	handler := server.New(server.Config{MediaDir: media, DataDir: filepath.Join(root, "data"), CacheDir: filepath.Join(root, "cache"), BackupDir: filepath.Join(root, "backups"), Lifecycle: t.Context()})
	id := apiItemsByTitle(t, handler)["Archive"]
	if id == "" {
		t.Fatal("private archive absent from library")
	}
	return handler, id, sentinel
}

func readerTrustUnavailable(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || response.Body.String() != "{\"error\":\"not found\"}\n" {
		t.Errorf("invalid reader = %d %q (%s), want exact not-found JSON", response.Code, response.Body.String(), response.Header().Get("Content-Type"))
	}
}

func readerTrustAssetRejected(t *testing.T, handler http.Handler, id, asset string, sentinel []byte) {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/read/"+id+"/asset/"+asset, nil)
	missing := apiCall(t, handler, "", http.MethodGet, "/read/missing/asset/missing.xhtml", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing-asset control = %d", missing.Code)
	}
	if response.Code != http.StatusNotFound || !bytes.Equal(response.Body.Bytes(), missing.Body.Bytes()) || response.Header().Get("Content-Type") != missing.Header().Get("Content-Type") {
		t.Errorf("rejected asset %q = %d %q, want complete canonical not-found body", asset, response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), sentinel) {
		t.Errorf("rejected asset %q returned outside-media sentinel bytes", asset)
	}
}

func readerTrustAssetEquals(t *testing.T, handler http.Handler, id, asset string, expected []byte) {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/read/"+id+"/asset/"+asset, nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), expected) {
		t.Errorf("asset %q = %d %q, want exact resource bytes", asset, response.Code, response.Body.String())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'none'") {
		t.Errorf("asset %q lost protected content headers: %v", asset, response.Header())
	}
}

func readerTrustPNG(t *testing.T, pixel color.RGBA) []byte {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, 1, 1))
	frame.SetRGBA(0, 0, pixel)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, frame); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func readerTrustProgressFixture(t *testing.T, malformed bool) (http.Handler, string, func() http.Handler) {
	t.Helper()
	root := t.TempDir()
	media := filepath.Join(root, "media")
	if err := os.Mkdir(media, 0o700); err != nil {
		t.Fatal(err)
	}
	href := "two.xhtml"
	if malformed {
		href = "../../outside.xhtml"
	}
	manifest := `<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="` + href + `" media-type="application/xhtml+xml"/>`
	entries := readerTrustEPUB(manifest, `<itemref idref="one"/><itemref idref="two"/>`, readerTrustEntry{name: "OEBPS/one.xhtml", body: []byte("<html>one</html>")}, readerTrustEntry{name: "OEBPS/two.xhtml", body: []byte("<html>two</html>")})
	archive := readerTrustArchive(t, "epub", entries)
	archivePath := filepath.Join(media, "Progress.epub")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("private progress outside-media sentinel")
	sentinelPath := filepath.Join(root, "outside.xhtml")
	if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		actual, err := os.ReadFile(archivePath)
		if err != nil || !bytes.Equal(actual, archive) {
			t.Errorf("progress source archive changed: %v", err)
		}
		outside, err := os.ReadFile(sentinelPath)
		if err != nil || !bytes.Equal(outside, sentinel) {
			t.Errorf("progress outside sentinel changed: %v", err)
		}
	})
	config := server.Config{MediaDir: media, DataDir: filepath.Join(root, "data"), CacheDir: filepath.Join(root, "cache"), BackupDir: filepath.Join(root, "backups"), Lifecycle: t.Context()}
	reopen := func() http.Handler { return server.New(config) }
	handler := reopen()
	id := apiItemsByTitle(t, handler)["Progress"]
	if id == "" {
		t.Fatal("private progress archive absent")
	}
	return handler, id, reopen
}

func readerTrustSavedProgress(t *testing.T, handler http.Handler, id string) []byte {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("generic item progress = %d %q", response.Code, response.Body.String())
	}
	var body struct {
		Item struct{ Progress json.RawMessage }
	}
	mustJSON(t, response, &body)
	if len(body.Item.Progress) == 0 {
		t.Fatal("generic item omitted progress")
	}
	return append([]byte(nil), body.Item.Progress...)
}
