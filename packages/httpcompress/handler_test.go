package httpcompress

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var page = strings.Repeat(`<article class="card"><a href="/item/0123456789abcdef"><img src="/artwork/0123456789abcdef" width="400" height="600" alt=""></a><h2>Arrival</h2></article>`, 64)

type observed struct {
	header http.Header
	length int64
}

func get(t *testing.T, server *httptest.Server, method string, header http.Header) (observed, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header = header
	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return observed{header: response.Header, length: response.ContentLength}, body
}

func gunzip(t *testing.T, body []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestHandlerCompressesSubtitlesAndKeepsTheRepresentationConsistent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		writer.Header().Set("ETag", `"page-1"`)
		for chunk := range strings.SplitSeq(page, "</article>") {
			_, _ = io.WriteString(writer, chunk+"</article>")
		}
	})))
	defer server.Close()
	response, body := get(t, server, http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "Cookie": {"session=1"}})
	if response.header.Get("Content-Encoding") != "gzip" || response.header.Get("Vary") != "Accept-Encoding" || response.header.Get("ETag") != `W/"page-1"` || response.length != -1 && response.length != int64(len(body)) {
		t.Fatalf("headers = %v", response.header)
	}
	if decoded := gunzip(t, body); !bytes.HasPrefix(decoded, []byte(page)) || len(body) >= len(decoded)/4 {
		t.Fatalf("decoded %d bytes from %d", len(decoded), len(body))
	}
}

func TestHandlerLeavesIneligibleResponsesUntouched(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		header   http.Header
		method   string
		response func(http.ResponseWriter)
	}{
		"no gzip":          {http.Header{"Accept-Encoding": {"br"}}, http.MethodGet, subtitlePage},
		"malformed accept": {http.Header{"Accept-Encoding": {"gzip;q=2"}}, http.MethodGet, subtitlePage},
		"html page": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(writer, page)
		}},
		"json": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, page)
		}},
		"range request": {http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}}, http.MethodGet, subtitlePage},
		"head":          {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodHead, subtitlePage},
		"partial content": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/vtt")
			writer.Header().Set("Content-Range", "bytes 0-1999/4000")
			writer.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(writer, page[:2000])
		}},
		"already encoded": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/css")
			writer.Header().Set("Content-Encoding", "br")
			_, _ = io.WriteString(writer, page)
		}},
		"media": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "video/mp4")
			_, _ = io.WriteString(writer, page)
		}},
		"playlist": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = io.WriteString(writer, page)
		}},
		"small subtitles": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/vtt")
			_, _ = io.WriteString(writer, page[:200])
		}},
		"small declared": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/vtt")
			writer.Header().Set("Content-Length", "20")
			_, _ = io.WriteString(writer, page[:20])
		}},
		"not found": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/vtt")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(writer, page)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				test.response(writer)
			})))
			defer server.Close()
			response, body := get(t, server, test.method, test.header)
			if response.header.Get("Content-Encoding") == "gzip" {
				t.Fatalf("compressed: %v", response.header)
			}
			if test.method == http.MethodGet && !strings.HasPrefix(page, string(body)) {
				t.Fatalf("body changed: %.40q", body)
			}
		})
	}
}

func subtitlePage(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	_, _ = io.WriteString(writer, page)
}

func TestHandlerFlushesCompressedAndEventStreams(t *testing.T) {
	t.Parallel()
	for contentType, encoding := range map[string]string{"text/vtt": "gzip", "text/event-stream": ""} {
		proceed := make(chan struct{})
		server := httptest.NewServer(Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", contentType)
			_, _ = io.WriteString(writer, "first")
			if err := http.NewResponseController(writer).Flush(); err != nil {
				t.Error(err)
			}
			<-proceed
			_, _ = io.WriteString(writer, page)
		})))
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		request.Header.Set("Accept-Encoding", "gzip")
		response, err := (&http.Transport{DisableCompression: true}).RoundTrip(request)
		if err != nil || response.Header.Get("Content-Encoding") != encoding {
			t.Fatalf("%s = %v %v", contentType, response, err)
		}
		reader := io.Reader(response.Body)
		if encoding == "gzip" {
			if reader, err = gzip.NewReader(response.Body); err != nil {
				t.Fatal(err)
			}
		}
		first := make([]byte, len("first"))
		if _, err := io.ReadFull(reader, first); err != nil || string(first) != "first" {
			t.Fatalf("%s flushed %q %v", contentType, first, err)
		}
		close(proceed)
		rest, err := io.ReadAll(reader)
		if err != nil || string(rest) != page {
			t.Fatalf("%s rest = %d bytes %v", contentType, len(rest), err)
		}
		_ = response.Body.Close()
		cancel()
		server.Close()
	}
}

type readerFromRecorder struct {
	*httptest.ResponseRecorder
	readFrom int
}

func (recorder *readerFromRecorder) ReadFrom(source io.Reader) (int64, error) {
	recorder.readFrom++
	return io.Copy(recorder.ResponseRecorder, source)
}

func TestHandlerKeepsZeroCopyPathsAndUnwraps(t *testing.T) {
	t.Parallel()
	recorder := &readerFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	var unwrapped http.ResponseWriter
	Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		unwrapped = writer.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
		writer.Header().Set("Content-Type", "video/mp4")
		_, _ = io.Copy(writer, struct{ io.Reader }{strings.NewReader(page)})
	})).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media", nil))
	if recorder.readFrom != 1 || recorder.Body.String() != page || unwrapped != http.ResponseWriter(recorder) {
		t.Fatalf("ReadFrom calls = %d, body %d bytes", recorder.readFrom, recorder.Body.Len())
	}
	compressed := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitles.vtt", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/vtt")
		writer.Header().Set("Content-Length", "8960")
		_, _ = io.Copy(writer, strings.NewReader(page))
	})).ServeHTTP(compressed, request)
	if compressed.Header().Get("Content-Encoding") != "gzip" || string(gunzip(t, compressed.Body.Bytes())) != page {
		t.Fatalf("copied VTT = %v", compressed.Header())
	}
}

func subtitleTrack() string {
	var builder strings.Builder
	builder.WriteString("WEBVTT\n\n")
	lines := []string{"Where are you going?", "We have to leave before the tide turns.", "I never said that.", "Keep the lights low and stay close.", "[distant thunder]"}
	for index := range 1500 {
		start := index * 2400
		fmt.Fprintf(&builder, "%02d:%02d:%02d.%03d --> %02d:%02d:%02d.%03d\n%s\n\n", start/3600000, start/60000%60, start/1000%60, start%1000, (start+2000)/3600000, (start+2000)/60000%60, (start+2000)/1000%60, (start+2000)%1000, lines[index*7%len(lines)])
	}
	return builder.String()
}

func BenchmarkHandlerSubtitleTrack(b *testing.B) {
	track := subtitleTrack()
	handler := Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		_, _ = io.WriteString(writer, track)
	}))
	for _, encoding := range []string{"identity", "gzip"} {
		b.Run(encoding, func(b *testing.B) {
			request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil)
			request.Header.Set("Accept-Encoding", encoding)
			var wire int
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				wire = response.Body.Len()
			}
			b.ReportMetric(float64(wire), "wire-bytes")
		})
	}
}
