package server_test

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var scimConfiguration = servertest.SCIMConfiguration[configuration.Snapshot]{
	AuthURL: "https://media.example:38128", Token: scimTestToken,
	Load: configuration.Load, SignIn: signInTestProfile, WebCall: requestWithCookie,
	NewHandler: func(directory string, configured configuration.Snapshot, token string, expires time.Time) http.Handler {
		return server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured, SCIM: server.SCIMConfig{Token: token, TokenExpiresAt: expires}})
	},
}
