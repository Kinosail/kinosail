package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var jellyfinOwnerSessions = servertest.JellyfinOwnerSessions{
	NewHandler: func(t *testing.T, mediaDir, dataDir string) http.Handler {
		return newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	},
	Set: configuration.Set, SignIn: signInTestProfile, WebCall: requestWithCookie,
	Call: jellyfinCall, Decode: decodeJellyfin, APIServer: apiServer, Enable: enableJellyfin, DisableMFA: disableTestMFA,
}
