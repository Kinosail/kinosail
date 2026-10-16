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

func get(t *testing.T, server *httptest.Server, method string, header http.Header) (*http.Response, []byte) {
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
	return response, body
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

func TestHandlerCompressesEligibleTextAndKeepsTheRepresentationConsistent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("ETag", `"page-1"`)
		for chunk := range strings.SplitSeq(page, "</article>") {
			_, _ = io.WriteString(writer, chunk+"</article>")
		}
	})))
	defer server.Close()
	response, body := get(t, server, http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "Sec-Fetch-Site": {"same-origin"}, "Cookie": {"session=1"}})
	if response.Header.Get("Content-Encoding") != "gzip" || response.Header.Get("Vary") != "Accept-Encoding" || response.Header.Get("ETag") != `W/"page-1"` || response.ContentLength != -1 && response.ContentLength != int64(len(body)) {
		t.Fatalf("headers = %v", response.Header)
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
		"no gzip":                       {http.Header{"Accept-Encoding": {"br"}}, http.MethodGet, htmlPage},
		"malformed accept":              {http.Header{"Accept-Encoding": {"gzip;q=2"}}, http.MethodGet, htmlPage},
		"cross-site":                    {http.Header{"Accept-Encoding": {"gzip"}, "Sec-Fetch-Site": {"cross-site"}}, http.MethodGet, htmlPage},
		"same-site":                     {http.Header{"Accept-Encoding": {"gzip"}, "Sec-Fetch-Site": {"same-site"}}, http.MethodGet, htmlPage},
		"cookie without fetch metadata": {http.Header{"Accept-Encoding": {"gzip"}, "Cookie": {"session=1"}}, http.MethodGet, htmlPage},
		"range request":                 {http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}}, http.MethodGet, htmlPage},
		"head":                          {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodHead, htmlPage},
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
		"small json": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"ok":true}`)
		}},
		"small declared": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/html")
			writer.Header().Set("Content-Length", "20")
			_, _ = io.WriteString(writer, page[:20])
		}},
		"not found": {http.Header{"Accept-Encoding": {"gzip"}}, http.MethodGet, func(writer http.ResponseWriter) {
			writer.Header().Set("Content-Type", "text/html")
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
			if response.Header.Get("Content-Encoding") == "gzip" {
				t.Fatalf("compressed: %v", response.Header)
			}
			if test.method == http.MethodGet && !strings.HasPrefix(page, string(body)) && !strings.HasPrefix(string(body), `{"ok"`) {
				t.Fatalf("body changed: %.40q", body)
			}
		})
	}
}

func htmlPage(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(writer, page)
}

func TestHandlerFlushesCompressedAndEventStreams(t *testing.T) {
	t.Parallel()
	for contentType, encoding := range map[string]string{"application/json": "gzip", "text/event-stream": ""} {
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

func libraryPage() string {
	var builder strings.Builder
	titles := []string{"Arrival", "Blade Runner 2049", "Dune: Part Two", "Everything Everywhere All at Once", "Mad Max: Fury Road", "Past Lives", "The Zone of Interest"}
	for index := range 400 {
		id := fmt.Sprintf("%016x", uint64(index+1)*0x9e3779b97f4a7c15)
		fmt.Fprintf(&builder, `<article class="card" data-item="%s"><a href="/item/%s"><img src="/artwork/%s?v=%d" width="400" height="600" decoding="async" loading="lazy" alt=""></a><h2>%s</h2><p>%d · %d min</p><progress value="%d" max="100"></progress></article>`, id, id, id, index*7919%1000, titles[index%len(titles)], 1990+index%35, 85+index%70, index*37%100)
	}
	return builder.String()
}

func BenchmarkHandlerHTMLPage(b *testing.B) {
	library := libraryPage()
	handler := Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, library)
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
