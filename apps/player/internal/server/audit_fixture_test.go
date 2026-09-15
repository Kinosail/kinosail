package server_test

import (
	"net/http"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var auditFixture = servertest.AuditFixture{
	New: func(media, data string) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	},
	SignIn: signInTestProfile, CookieRequest: requestWithCookie, ProfileID: storedProfileID,
}
