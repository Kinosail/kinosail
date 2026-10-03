package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestBrowserSessionCookieIsolatedFromSiblingApp(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38128"})
	servertest.AssertBrowserSessionCookieIsolation(t, handler, "https://localhost:38128", "__Host-kinosail_subtitles_session", "__Host-kinosail_player_session")
}

func TestPasskeyLoginCookiesIsolatedFromSiblingApp(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38128"})
	servertest.AssertPasskeyCookieIsolation(t, handler, "https://localhost:38128", "__Host-kinosail_subtitles_session", "kinosail_subtitles_passkey", "kinosail_player_passkey")
}

func TestLegacyCookieMigratesThroughValidatedAppSession(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38128"})
	servertest.AssertLegacyBrowserCookieMigration(t, handler, "https://localhost:38128", "__Host-kinosail_subtitles_session")
}
