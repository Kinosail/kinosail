package server_test

import (
	"fmt"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerCanRunLocalTranscoderCheckThroughWebAndAPI(t *testing.T) {
	t.Parallel()
	data, tools := t.TempDir(), t.TempDir()
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments))
	fixture, err := os.ReadFile(ffmpeg)
	if err != nil {
		t.Fatal(err)
	}
	fixture = append(fixture, []byte("for output; do case \"$output\" in */index.m3u8) directory=${output%/*}; mkdir -p \"$directory\"; "+mp4fixture.Shell(mp4fixture.Initialization(320, 180, "h264", "aac", ""))+" > \"$directory/init.mp4\";; esac; done\n")...)
	writeExecutable(t, ffmpeg, string(fixture))
	handler := server.New(server.Config{DataDir: data, FFmpeg: ffmpeg, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder/test", nil)
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings#transcoder" {
		t.Fatalf("web test = %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	settings := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(settings, request)
	mustContainAll(t, settings.Body.String(), "Video conversion is ready", "Software (libx264)", "Check again", "The basic video tools work")

	session := struct{ Token string }{owner.Value}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/transcoder/test", nil), http.StatusUnauthorized, "authentication required")
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password", "owner": false}), http.StatusCreated)
	viewerLogin := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var viewer struct{ Token string }
	mustJSON(t, viewerLogin, &viewer)
	enrollTestAPIFactor(t, handler, viewer.Token)
	assertAPIBody(t, apiCall(t, handler, viewer.Token, http.MethodPost, "/api/v1/transcoder/test", nil), http.StatusForbidden, "Owner access required")
	check := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/transcoder/test", nil)
	assertAPIBody(t, check, http.StatusOK, `"status":"passed"`, `"accelerator":"none"`, `"backend":"Software (libx264)"`)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"transcoderTest":{"status":"passed"`)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "speed", "accelerator": "none"}), http.StatusOK)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"transcoderTest":{"status":"not-run"`)

	used, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"testsrc2=size=640x360:rate=24:duration=1", "-c:v libx264", "-c:a aac", "-vf scale="} {
		if !strings.Contains(string(used), expected) {
			t.Fatalf("transcoder check lacks %q: %q", expected, used)
		}
	}
}

func TestLocalTranscoderCheckReturnsActionableFailure(t *testing.T) {
	t.Parallel()
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\ncase \"$*\" in *testsrc2*) exit 0;; *) exit 1;; esac\n")
	handler := server.New(server.Config{DataDir: t.TempDir(), FFmpeg: ffmpeg, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	session := struct{ Token string }{owner.Value}

	check := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/transcoder/test", nil)
	assertAPIBody(t, check, http.StatusServiceUnavailable, `"status":"failed"`, `"stage":"encode"`, "Choose Automatic or Software")
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/settings", nil), http.StatusOK, "Video check failed", "Choose Automatic or Processor", "Video compatibility")
}
