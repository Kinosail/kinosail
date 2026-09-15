package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var localeWebFixture = servertest.LocaleWebFixture{
	NewHandler: func(t *testing.T, requireAuth bool) http.Handler {
		t.Helper()
		return server.New(server.Config{DataDir: t.TempDir(), RequireAuth: requireAuth})
	},
	Server: apiServer,
}
