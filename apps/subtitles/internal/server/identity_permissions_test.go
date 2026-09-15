package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestViewerPolicyRestrictsLibrariesRemoteScheduleAndTranscoding(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	setupPolicyMedia(t, mediaDir, dataDir)
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true, ProxyToken: "proxy-capability"})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	policy := "name=Sam&password=viewer-password&rating=all&libraries=Movies&remote=true&transcode=false"
	response := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", policy, owner)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	viewer := signInTestProfile(t, handler, "/login", "name=Sam&password=viewer-password")
	profileID := storedProfileID(t, dataDir, "Sam")
	home := requestWithCookie(t, handler, http.MethodGet, "/", "", viewer)
	if !strings.Contains(home.Body.String(), "Arrival") || strings.Contains(home.Body.String(), "Pilot") {
		t.Fatalf("restricted library = %q", home.Body.String())
	}
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request := requestWithCookieRequest(t, http.MethodGet, "/hls/"+id+"/index.m3u8", "", viewer)
	denied := serveRequest(handler, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("transcode = %d %q", denied.Code, denied.Body.String())
	}
	start, end := time.Now().Add(2*time.Hour).Format("15:04"), time.Now().Add(3*time.Hour).Format("15:04")
	policy = "id=" + profileID + "&rating=all&libraries=Movies&start=" + start + "&end=" + end
	request = requestWithCookieRequest(t, http.MethodPost, "/settings/profiles/permissions", policy, owner)
	serveRequest(handler, request)
	scheduled := requestWithCookie(t, handler, http.MethodGet, "/", "", viewer)
	if scheduled.Code != http.StatusForbidden {
		t.Fatalf("scheduled access = %d %q", scheduled.Code, scheduled.Body.String())
	}
	policy = "id=" + profileID + "&rating=all&libraries=Movies&start=&end="
	request = requestWithCookieRequest(t, http.MethodPost, "/settings/profiles/permissions", policy, owner)
	serveRequest(handler, request)
	remote := requestWithCookieRequest(t, http.MethodGet, "/", "", viewer)
	remote.Header.Set("X-Kinosail-Remote", "true")
	remote.Header.Set("X-Kinosail-Proxy-Token", "proxy-capability")
	remoteResponse := serveRequest(handler, remote)
	if remoteResponse.Code != http.StatusForbidden {
		t.Fatalf("remote access = %d %q", remoteResponse.Code, remoteResponse.Body.String())
	}
}

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
