package server_test

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestBrowserSessionUseDoesNotRewriteStateAndRevocationSurvivesRestart(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	viewer := addAndSignInViewer(t, handler, owner, "Sam", "viewer-password")
	if response := getWithCookie(t, handler, "/api/v1/me", viewer); response.Code != http.StatusOK {
		t.Fatalf("viewer request = %d %q", response.Code, response.Body.String())
	}
	before := storedState(t, dataDir, "sessions.json")
	time.Sleep(1100 * time.Millisecond)
	for range 5 {
		if response := getWithCookie(t, handler, "/api/v1/me", viewer); response.Code != http.StatusOK {
			t.Fatalf("repeated viewer request = %d %q", response.Code, response.Body.String())
		}
	}
	if after := storedState(t, dataDir, "sessions.json"); !bytes.Equal(before, after) {
		t.Fatalf("repeated browser requests rewrote sessions:\nbefore=%s\nafter=%s", before, after)
	}
	if revoke := requestWithCookie(t, handler, http.MethodPost, "/settings/sessions/revoke", "", owner); revoke.Code != http.StatusSeeOther {
		t.Fatalf("revoke other sessions = %d %q", revoke.Code, revoke.Body.String())
	}
	if denied := getWithCookie(t, handler, "/api/v1/me", viewer); denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked viewer before restart = %d %q", denied.Code, denied.Body.String())
	}
	restarted := server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	if denied := getWithCookie(t, restarted, "/api/v1/me", viewer); denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked viewer after restart = %d %q", denied.Code, denied.Body.String())
	}
	if allowed := getWithCookie(t, restarted, "/api/v1/me", owner); allowed.Code != http.StatusOK {
		t.Fatalf("owner after restart = %d %q", allowed.Code, allowed.Body.String())
	}
}
