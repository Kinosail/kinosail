package httpcompress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func subtitleTrack() []byte {
	var builder strings.Builder
	builder.WriteString("WEBVTT\n\n")
	lines := []string{"Where are you going?", "We have to leave before the tide turns.", "I never said that.", "Keep the lights low and stay close.", "[distant thunder]"}
	for index := range 1500 {
		start := index * 2400
		fmt.Fprintf(&builder, "%02d:%02d:%02d.%03d --> %02d:%02d:%02d.%03d\n%s\n\n", start/3600000, start/60000%60, start/1000%60, start%1000, (start+2000)/3600000, (start+2000)/60000%60, (start+2000)/1000%60, (start+2000)%1000, lines[index*7%len(lines)])
	}
	return []byte(builder.String())
}

func writeTrack(t *testing.T, method string, header http.Header, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "/subtitle/abc", nil)
	request.Header = header
	response := httptest.NewRecorder()
	response.Header().Set("ETag", `"track-1"`)
	response.Header().Set("Content-Length", "999999")
	WriteWebVTT(response, request, body)
	return response
}

func TestWriteWebVTTCompressesEligibleTracks(t *testing.T) {
	t.Parallel()
	track := subtitleTrack()
	response := writeTrack(t, http.MethodGet, http.Header{"Accept-Encoding": {"br;q=1, gzip;q=0.8"}}, track)
	header := response.Header()
	if response.Code != http.StatusOK || header.Get("Content-Encoding") != "gzip" || header.Get("Content-Type") != "text/vtt; charset=utf-8" || header.Get("Vary") != "Accept-Encoding" || header.Get("ETag") != `W/"track-1"` || header.Get("Content-Length") != "" {
		t.Fatalf("compressed track = %d %v", response.Code, header)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decoded, track) || response.Body.Len() >= len(track)/4 {
		t.Fatalf("decoded %d bytes from %d: %v", len(decoded), response.Body.Len(), err)
	}
}

func TestWriteWebVTTServesIdentityWhenGzipIsNotSafeOrUseful(t *testing.T) {
	t.Parallel()
	track := subtitleTrack()
	for name, test := range map[string]struct {
		method string
		header http.Header
		body   []byte
	}{
		"no gzip":          {http.MethodGet, http.Header{"Accept-Encoding": {"br"}}, track},
		"gzip refused":     {http.MethodGet, http.Header{"Accept-Encoding": {"gzip;q=0"}}, track},
		"malformed accept": {http.MethodGet, http.Header{"Accept-Encoding": {"gzip;q=2"}}, track},
		"range":            {http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-99"}}, track},
		"etag condition":   {http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "If-None-Match": {`"track-1"`}}, track},
		"date condition":   {http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "If-Modified-Since": {"Mon, 01 Jan 2024 00:00:00 GMT"}}, track},
		"head":             {http.MethodHead, http.Header{"Accept-Encoding": {"gzip"}}, track},
		"small":            {http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}}, track[:MinimumSize-1]},
	} {
		response := writeTrack(t, test.method, test.header, test.body)
		header := response.Header()
		if response.Code != http.StatusOK || header.Get("Content-Encoding") != "" || header.Get("ETag") != `"track-1"` || header.Get("Vary") != "Accept-Encoding" || header.Get("Content-Length") != strconv.Itoa(len(test.body)) || !bytes.Equal(response.Body.Bytes(), test.body) {
			t.Fatalf("%s = %d %v, %d bytes", name, response.Code, header, response.Body.Len())
		}
	}
}

func BenchmarkWriteWebVTT(b *testing.B) {
	track := subtitleTrack()
	for _, encoding := range []string{"identity", "gzip"} {
		b.Run(encoding, func(b *testing.B) {
			request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/subtitle/abc", nil)
			request.Header.Set("Accept-Encoding", encoding)
			var wire int
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				WriteWebVTT(response, request, track)
				wire = response.Body.Len()
			}
			b.ReportMetric(float64(wire), "wire-bytes")
		})
	}
}
