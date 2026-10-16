package publicgateway

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/httpcompress"
)

func TestGatewayPassesApplicationCompressionThroughUnchanged(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "gateway-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "http.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	page := "WEBVTT\n\n" + strings.Repeat("00:00:01.000 --> 00:00:02.000\nShared with you\n\n", 60)
	var mu sync.Mutex
	var offered []string
	backend := &http.Server{Handler: httpcompress.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		offered = append(offered, r.Header.Get("Accept-Encoding"))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})), ReadHeaderTimeout: time.Second}
	go func() { _ = backend.Serve(listener) }()
	t.Cleanup(func() { _ = backend.Close() })
	for _, encoding := range []string{"gzip", ""} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://family.duckdns.org/share", nil)
		r.URL.Scheme, r.URL.Host = "", ""
		if encoding != "" {
			r.Header.Set("Accept-Encoding", encoding)
		}
		w := httptest.NewRecorder()
		Handler("family.duckdns.org", path).ServeHTTP(w, r)
		body := w.Body.Bytes()
		if encoding == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatalf("gateway did not pass gzip through: %v %v", w.Header(), err)
			}
			if body, err = io.ReadAll(reader); err != nil {
				t.Fatal(err)
			}
		}
		if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != encoding || string(body) != page {
			t.Fatalf("Accept-Encoding %q = %d %v", encoding, w.Code, w.Header())
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(offered, "|") != "gzip|" {
		t.Fatalf("application saw Accept-Encoding %q", offered)
	}
}
