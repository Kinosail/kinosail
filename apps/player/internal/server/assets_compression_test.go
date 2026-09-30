package server_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticBundlesNegotiateCompressionWithoutChangingContent(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	for _, path := range []string{"/static/app.css?v=speed", "/static/player.js?v=speed", "/static/hls.min.js?v=speed"} {
		plain := httptest.NewRecorder()
		handler.ServeHTTP(plain, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		for _, encoding := range []string{"gzip", "br, gzip;q=0.8", "GZip; q=1.000", "*;q=0.5"} {
			assertCompressedAsset(t, handler, path, encoding, plain)
		}
	}
}

func assertCompressedAsset(t *testing.T, handler http.Handler, path, encoding string, plain *httptest.ResponseRecorder) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Accept-Encoding", encoding)
	compressed := httptest.NewRecorder()
	handler.ServeHTTP(compressed, request)
	if compressed.Code != http.StatusOK || compressed.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(compressed.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("%s encoding %q = %d %v", path, encoding, compressed.Code, compressed.Header())
	}
	compressedSize := compressed.Body.Len()
	t.Logf("%s: identity %d bytes, gzip %d bytes (%.1f%% reduction)", path, plain.Body.Len(), compressedSize, 100*(1-float64(compressedSize)/float64(plain.Body.Len())))
	reader, err := gzip.NewReader(compressed.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plain.Body.Bytes()) || compressed.Header().Get("Content-Type") != plain.Header().Get("Content-Type") || compressed.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("%s compression changed the delivered asset", path)
	}
	if compressedSize >= plain.Body.Len()/2 {
		t.Fatalf("%s compressed size %d, plain %d", path, compressedSize, plain.Body.Len())
	}
}

func TestStaticCompressionRespectsExclusionsAndBounds(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	for _, encoding := range []string{"", "br", "gzip;q=0", "gzip;q=0, *;q=1", "gzip;q=0.5, identity;q=1"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil)
		request.Header.Set("Accept-Encoding", encoding)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Content-Encoding") != "" || response.Header().Get("Cache-Control") != "public, max-age=86400" {
			t.Fatalf("encoding %q = %d %v", encoding, response.Code, response.Header())
		}
	}
	for _, path := range []string{"/api/v1/me", "/static/cinema-backdrop.jpg", "/static/icon-192.png"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		request.Header.Set("Accept-Encoding", "gzip")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Header().Get("Content-Encoding") != "" {
			t.Fatalf("unexpected compression at %s", path)
		}
	}
}

func TestStaticCompressionRejectsMalformedOrUnacceptableNegotiation(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	for _, test := range []struct {
		headers []string
		status  int
	}{
		{[]string{"gzip;q=0, gzip;q=1"}, 400},
		{[]string{"gzip;q=oops"}, 400},
		{[]string{"gzip;q=1.1"}, 400},
		{[]string{"gzip;q=-1"}, 400},
		{[]string{"gzip;q=0.0001"}, 400},
		{[]string{"gzip;unknown=1"}, 400},
		{[]string{"gzip;q=1;other=1"}, 400},
		{[]string{"gzip, bad coding"}, 400},
		{[]string{strings.Repeat(" ", 4097)}, 400},
		{[]string{"gzip, ;q=0"}, 400},
		{[]string{";unknown=1"}, 400},
		{strings.Split(strings.Repeat("br,", 17), ","), 400},
		{[]string{"a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,gzip"}, 400},
		{[]string{"gzip;q=0", "identity;q=0"}, 406},
		{[]string{"*;q=0"}, 406},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js?v=speed", nil)
		request.Header["Accept-Encoding"] = test.headers
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status || response.Body.Len() != 0 || response.Header().Get("Content-Encoding") != "" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("headers %q = %d, %d bytes, %v", test.headers, response.Code, response.Body.Len(), response.Header())
		}
	}
}

func TestStaticCompressionHEADAndMultipleHeaderLines(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequestWithContext(t.Context(), method, "/static/player.js?v=speed", nil)
		request.Header.Add("Accept-Encoding", "br;q=1")
		request.Header.Add("Accept-Encoding", "gzip;q=0.8, identity;q=0")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 || response.Header().Get("Content-Encoding") != "gzip" || response.Header().Get("Content-Length") == "" || method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatalf("%s = %d, %d bytes, %v", method, response.Code, response.Body.Len(), response.Header())
		}
	}
}
