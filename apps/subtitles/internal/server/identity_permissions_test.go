package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupPolicyMedia(t *testing.T, mediaDir, dataDir string) {
	t.Helper()
	for _, folder := range []string{"Movies", "Shows"} {
		if err := os.Mkdir(filepath.Join(mediaDir, folder), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Movies", "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Shows", "Pilot S01E01.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(`{"name":"Kinosail","libraries":["Movies","Shows"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requestWithCookieRequest(t *testing.T, method, path, body string, cookie *http.Cookie) *http.Request {
	t.Helper()
	request := requestWithBody(t, method, path, body)
	request.AddCookie(cookie)
	return request
}

func requestWithBody(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, "http://kinosail.test"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func serveRequest(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
