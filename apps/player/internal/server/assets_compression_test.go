package server_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestFixedPlayerScriptsKeepBytesAndRequestPolicyAcrossApplications(t *testing.T) {
	t.Parallel()
	anonymous := assetContracts.NewHandler("", "", false)
	authenticated := assetContracts.NewHandler("", "", true)
	for _, path := range []string{
		"/static/htmx.min.js", "/static/hls.min.js", "/static/player.js",
		"/static/downloads.js", "/static/main.kinosail.bundle.js", "/static/pwa.js",
		"/static/supporter.js", "/static/theme.js", "/static/quick-connect.js",
		"/static/connect.js", "/static/passkeys.js",
	} {
		t.Run(path, func(t *testing.T) {
			plain := fixedScriptResponse(t, anonymous, http.MethodGet, path, "")
			other := fixedScriptResponse(t, authenticated, http.MethodGet, path, "")
			if !bytes.Equal(plain.Body.Bytes(), other.Body.Bytes()) || plain.Body.Len() == 0 {
				t.Fatal("app configuration changed public script bytes")
			}
			for _, handler := range []http.Handler{anonymous, authenticated} {
				assertFixedScriptRequests(t, handler, path, plain.Body.Bytes())
			}
		})
	}
}

func fixedScriptResponse(t *testing.T, handler http.Handler, method, path, encoding string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	request.Header.Set("Accept-Encoding", encoding)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("%s %s: status/type %d %q", method, path, response.Code, response.Header().Get("Content-Type"))
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(response.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatal("script lost application browser security policy")
	}
	return response
}

func assertFixedScriptRequests(t *testing.T, handler http.Handler, path string, identity []byte) {
	t.Helper()
	for _, query := range []string{"", "?v=fixed-contract"} {
		response := fixedScriptResponse(t, handler, http.MethodGet, path+query, "gzip")
		decoded := response.Body.Bytes()
		if response.Header().Get("Content-Encoding") == "gzip" {
			decoded = decodeFixedScript(t, response)
		}
		if !bytes.Equal(decoded, identity) {
			t.Fatal("encoding negotiation changed script bytes")
		}
		expectedCache := "public, max-age=86400"
		if query != "" {
			expectedCache = "public, max-age=31536000, immutable"
		}
		assertFixedScriptHeaders(t, handler, path+query, expectedCache, response)
	}
	plain := fixedScriptResponse(t, handler, http.MethodGet, path, "identity")
	if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(plain.Body.Bytes(), identity) {
		t.Fatal("a prior gzip request leaked encoding into identity response")
	}
}

func assertFixedScriptHeaders(t *testing.T, handler http.Handler, path, expectedCache string, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != expectedCache {
		t.Fatalf("%s Cache-Control = %q, want %q", path, response.Header().Get("Cache-Control"), expectedCache)
	}
	if response.Header().Get("Content-Length") != strconv.Itoa(response.Body.Len()) {
		t.Fatalf("%s Content-Length = %q, want %d delivered bytes", path, response.Header().Get("Content-Length"), response.Body.Len())
	}
	head := fixedScriptResponse(t, handler, http.MethodHead, path, "gzip")
	if head.Body.Len() != 0 || head.Header().Get("Content-Encoding") != response.Header().Get("Content-Encoding") || head.Header().Get("Content-Length") != response.Header().Get("Content-Length") {
		t.Fatal("HEAD changed encoding/length or returned bytes")
	}
}

func decodeFixedScript(t *testing.T, response *httptest.ResponseRecorder) []byte {
	t.Helper()
	if !strings.Contains(response.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatal("compressed script lacks cache negotiation boundary")
	}
	reader, err := gzip.NewReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("gzip round trip: %v / %v", err, closeErr)
	}
	return decoded
}

func TestServiceWorkerKeepsItsSeparateScopeAndCachePolicy(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	response := fixedScriptResponse(t, handler, http.MethodGet, "/service-worker.js?v=fixed-contract", "gzip")
	if response.Body.Len() == 0 || response.Header().Get("Content-Encoding") != "" || response.Header().Get("Cache-Control") != "no-cache" || response.Header().Get("Service-Worker-Allowed") != "/" {
		t.Fatal("fixed script initialization changed service worker delivery policy")
	}
}
