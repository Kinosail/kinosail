package server_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubtitleExportCompressesOnlyWebVTT(t *testing.T) {
	t.Parallel()
	var track strings.Builder
	for cue := range 120 {
		fmt.Fprintf(&track, "%d\n00:%02d:%02d,000 --> 00:%02d:%02d,500\nThe tide turns at line %d.\n\n", cue+1, cue/60, cue%60, cue/60, cue%60, cue)
	}
	handler, base, _ := subtitleInspectorFixture(t, track.String())
	export := func(format, encoding string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, base+"/export?language=en&format="+format, nil)
		request.Header.Set("Accept-Encoding", encoding)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	plain := export("vtt", "identity")
	compressed := export("vtt", "gzip")
	if plain.Code != http.StatusOK || !strings.HasPrefix(plain.Body.String(), "WEBVTT") || compressed.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(strings.Join(compressed.Header().Values("Vary"), ","), "Accept-Encoding") {
		t.Fatalf("vtt export = %d %v / %v", plain.Code, plain.Header(), compressed.Header())
	}
	reader, err := gzip.NewReader(compressed.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decoded, plain.Body.Bytes()) || compressed.Body.Len() >= plain.Body.Len()/2 {
		t.Fatalf("decoded export differs or did not shrink: %d of %d bytes, %v", compressed.Body.Len(), plain.Body.Len(), err)
	}
	if srt := export("srt", "gzip"); srt.Code != http.StatusOK || srt.Header().Get("Content-Encoding") != "" {
		t.Fatalf("srt export = %d %v", srt.Code, srt.Header())
	}
}
