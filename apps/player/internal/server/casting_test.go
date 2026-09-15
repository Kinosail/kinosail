package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func castFixture(t *testing.T) (http.Handler, string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Song.mp3"), []byte("receiver media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: directory, AuthURL: "http://192.168.1.10:8080"})
	response := castRequest(t, handler, http.MethodGet, "/api/v1/library", "")
	var library struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &library); err != nil || len(library.Items) != 1 {
		t.Fatalf("library status=%d", response.Code)
	}
	return handler, library.Items[0].ID
}

func castRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	if strings.HasPrefix(path, "/") {
		path = "http://192.168.1.10:8080" + path
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCastGrantScopesDeliveryAndRevocation(t *testing.T) {
	handler, id := castFixture(t)
	created := castRequest(t, handler, http.MethodPost, "/api/v1/items/"+id+"/cast", `{"protocol":"google-cast","position":0}`)
	var session struct{ ID, URL, ContentType string }
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil || created.Code != 201 || session.ID == "" || session.ContentType != "audio/mpeg" {
		t.Fatalf("grant status=%d", created.Code)
	}
	assertCastDeliveryMethods(t, handler, session.URL)
	assertCastRange(t, handler, session.URL)
	for _, target := range []string{strings.Split(session.URL, "?")[0], session.URL + "&unknown=1", strings.Replace(session.URL, "/media?", "/hls/index.m3u8?", 1), strings.Replace(session.URL, "/media?", "/subtitles/1?", 1)} {
		if response := castRequest(t, handler, http.MethodGet, target, ""); response.Code != 404 {
			t.Fatalf("out-of-scope delivery=%d", response.Code)
		}
	}
	if response := castRequest(t, server.Remote(handler), http.MethodGet, session.URL, ""); response.Code != 404 {
		t.Fatalf("remote grant delivery=%d", response.Code)
	}
	if ended := castRequest(t, handler, http.MethodDelete, "/api/v1/cast/sessions/"+session.ID, ""); ended.Code != 204 {
		t.Fatalf("revoke=%d", ended.Code)
	}
	if response := castRequest(t, handler, http.MethodGet, session.URL, ""); response.Code != 404 {
		t.Fatalf("revoked delivery=%d", response.Code)
	}
}

func TestCastInvalidRequestsDoNotConsumeGrantCapacity(t *testing.T) {
	handler, id := castFixture(t)
	endpoint := "/api/v1/items/" + id + "/cast"
	for _, body := range []string{`{}`, `{"protocol":"unknown"}`, `{"protocol":"google-cast","position":-1}`, `{"protocol":"google-cast","position":31536001}`, `{"protocol":"google-cast","position":0,"deviceId":"other"}`, `{"protocol":"google-cast","position":0,"url":"http://127.0.0.1"}`, `{"protocol":"dlna","deviceId":"http://192.168.1.20/control"}`, `{"protocol":"google-cast","playbackToken":"invalid"}`, `{"protocol":"google-cast","playbackToken":"` + strings.Repeat("x", 8193) + `"}`} {
		if response := castRequest(t, handler, http.MethodPost, endpoint, body); response.Code != 400 {
			t.Fatalf("invalid request=%d", response.Code)
		}
	}
	for index := 0; index < 4; index++ {
		if response := castRequest(t, handler, http.MethodPost, endpoint, `{"protocol":"google-cast","position":0}`); response.Code != 201 {
			t.Fatalf("valid grant %d status=%d", index, response.Code)
		}
	}
	if response := castRequest(t, handler, http.MethodPost, endpoint, `{"protocol":"google-cast","position":0}`); response.Code != 400 {
		t.Fatalf("unbounded grants=%d", response.Code)
	}
}

func assertCastDeliveryMethods(t *testing.T, handler http.Handler, url string) {
	t.Helper()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		response := castRequest(t, handler, method, url, "")
		want := 200
		if method == http.MethodOptions {
			want = 204
		}
		if response.Code != want || response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("receiver %s status=%d", method, response.Code)
		}
	}
}

func assertCastRange(t *testing.T, handler http.Handler, url string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	request.Header.Set("Range", "bytes=0-3")
	partial := httptest.NewRecorder()
	handler.ServeHTTP(partial, request)
	if partial.Code != http.StatusPartialContent || partial.Body.String() != "rece" {
		t.Fatalf("range status=%d", partial.Code)
	}
}
