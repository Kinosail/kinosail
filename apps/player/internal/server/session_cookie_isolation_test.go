package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestBrowserSessionCookieIsolatedFromSiblingApp(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38127"})
	servertest.AssertBrowserSessionCookieIsolation(t, handler, "https://localhost:38127", "__Host-kinosail_player_session", "__Host-kinosail_subtitles_session")
}

func TestPasskeyLoginCookiesIsolatedFromSiblingApp(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38127"})
	servertest.AssertPasskeyCookieIsolation(t, handler, "https://localhost:38127", "__Host-kinosail_player_session", "kinosail_player_passkey", "kinosail_subtitles_passkey")
}

func TestLegacyCookieMigratesThroughValidatedAppSession(t *testing.T) {
	t.Parallel()
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://localhost:38127"})
	servertest.AssertLegacyBrowserCookieMigration(t, handler, "https://localhost:38127", "__Host-kinosail_player_session")
}
