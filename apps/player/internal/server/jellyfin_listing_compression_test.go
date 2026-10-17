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

func TestJellyfinListingsCompressWithoutChangingDecodedBytes(t *testing.T) {
	t.Parallel()
	handler, sessions, ids := jellyfinShapeServer(t)
	placeholders := make([]string, 0, 2*len(ids))
	for name, id := range ids {
		placeholders = append(placeholders, "{"+name+"}", id)
	}
	expand := strings.NewReplacer(placeholders...)
	compressed := 0
	for _, session := range sessions {
		for _, path := range session.paths {
			target := expand.Replace(path)
			plain := jellyfinEncodedCall(t, handler, target, "", session.token)
			encoded := jellyfinEncodedCall(t, handler, target, "gzip, br;q=0.5", session.token)
			if plain.Header().Get("Content-Encoding") != "" || !strings.Contains(plain.Header().Get("Vary"), "Accept-Encoding") {
				t.Fatalf("%s %s identity headers = %v", session.name, path, plain.Header())
			}
			if encoded.Code != plain.Code || encoded.Header().Get("Content-Type") != plain.Header().Get("Content-Type") || encoded.Header().Get("Cache-Control") != "no-store" || !strings.Contains(encoded.Header().Get("Vary"), "Accept-Encoding") {
				t.Fatalf("%s %s encoded = %d %v, identity = %d %v", session.name, path, encoded.Code, encoded.Header(), plain.Code, plain.Header())
			}
			body := encoded.Body.Bytes()
			if plain.Body.Len() >= 1024 {
				if encoded.Header().Get("Content-Encoding") != "gzip" || encoded.Header().Get("Content-Length") != "" || len(body) >= plain.Body.Len() {
					t.Fatalf("%s %s large listing = %d bytes %v, identity %d bytes", session.name, path, len(body), encoded.Header(), plain.Body.Len())
				}
				body = gunzipJellyfin(t, body)
				compressed++
			} else if encoded.Header().Get("Content-Encoding") != "" {
				t.Fatalf("%s %s small listing was encoded: %v", session.name, path, encoded.Header())
			}
			if !bytes.Equal(body, plain.Body.Bytes()) {
				t.Fatalf("%s %s decoded body differs\n got: %q\nwant: %q", session.name, path, body, plain.Body.String())
			}
		}
	}
	if compressed == 0 {
		t.Fatal("no Jellyfin listing reached the compression size")
	}
}

func jellyfinEncodedCall(t *testing.T, handler http.Handler, path, encoding, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Device="iPhone", DeviceId="test", Version="1.5", Token="`+token+`"`)
	if encoding != "" {
		request.Header.Set("Accept-Encoding", encoding)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func gunzipJellyfin(t *testing.T, body []byte) []byte {
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
