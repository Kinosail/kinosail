package publicgateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func gatewayFront(tb testing.TB, path string) *httptest.Server {
	tb.Helper()
	gateway := Handler("family.example", path)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, HandshakeComplete: true}
		r.Host = "family.example"
		gateway.ServeHTTP(w, r)
	}))
	tb.Cleanup(front.Close)
	return front
}

func TestGatewayStreamsEventsAndUnknownLengthResponsesImmediately(t *testing.T) {
	for name, contentType := range map[string]string{"server-sent events": "text/event-stream", "growing playlist": "application/vnd.apple.mpegurl"} {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			path := unixSocketServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = io.WriteString(w, "first\n")
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-time.After(5 * time.Second):
				}
				_, _ = io.WriteString(w, "second\n")
			}))
			front := gatewayFront(t, path)
			defer close(release)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, front.URL+"/events", nil)
			request.RequestURI = ""
			response, err := front.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if line, err := bufio.NewReader(response.Body).ReadString('\n'); err != nil || line != "first\n" {
				t.Fatalf("gateway buffered a streaming response: %q, %v", line, err)
			}
		})
	}
}

func TestGatewayDeliversKnownLengthMediaIntact(t *testing.T) {
	media := bytes.Repeat([]byte("kinosail-media-"), 1<<18)
	path := unixSocketServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", strconv.Itoa(len(media)))
		_, _ = w.Write(media)
	}))
	front := gatewayFront(t, path)
	for range 3 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, front.URL+"/media", nil)
		request.RequestURI = ""
		response, err := front.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		received, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil || response.ContentLength != int64(len(media)) || sha256.Sum256(received) != sha256.Sum256(media) || !strings.HasPrefix(response.Header.Get("Content-Type"), "video/mp4") {
			t.Fatalf("media response = %d bytes, length %d, %v", len(received), response.ContentLength, err)
		}
	}
}

func BenchmarkGatewayProxiesKnownLengthMedia(b *testing.B) {
	media := bytes.Repeat([]byte{7}, 64<<20)
	path := unixSocketServer(b, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", strconv.Itoa(len(media)))
		_, _ = w.Write(media)
	}))
	front := gatewayFront(b, path)
	b.SetBytes(int64(len(media)))
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, front.URL+"/media", nil)
		request.RequestURI = ""
		response, err := front.Client().Do(request)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			b.Fatal(err)
		}
		_ = response.Body.Close()
	}
}
