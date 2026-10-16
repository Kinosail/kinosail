package downloads

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type fileReaderRecorder struct {
	*httptest.ResponseRecorder
	fromFile bool
}

func (recorder *fileReaderRecorder) ReadFrom(source io.Reader) (int64, error) {
	inner := source
	if limited, ok := source.(*io.LimitedReader); ok {
		inner = limited.R
	}
	_, recorder.fromFile = inner.(*os.File)
	return io.Copy(recorder.ResponseRecorder, source)
}

func TestServeHandsTheFileToTheConnectionForSendfile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "film.mp4")
	content := []byte("0123456789")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	job := Job{Title: "Film", File: path, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}
	for _, test := range []struct {
		value, body string
		code        int
	}{{"", "0123456789", http.StatusOK}, {"bytes=2-5", "2345", http.StatusPartialContent}} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil)
		request.Header.Set("Range", test.value)
		response := &fileReaderRecorder{ResponseRecorder: httptest.NewRecorder()}
		if err := Serve(response, request, job); err != nil {
			t.Fatal(err)
		}
		if response.Code != test.code || response.Body.String() != test.body || !response.fromFile {
			t.Fatalf("range %q = %d %q, file reached connection: %v", test.value, response.Code, response.Body.String(), response.fromFile)
		}
	}
}
