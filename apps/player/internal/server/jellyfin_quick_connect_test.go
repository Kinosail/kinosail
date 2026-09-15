package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestJellyfinQuickConnect(t *testing.T) {
	servertest.AssertJellyfinQuickConnect(t, servertest.JellyfinQuickConnectFixture{
		NewHandler: func(t *testing.T, ttl time.Duration) http.Handler {
			return newJellyfinServer(t, server.Config{RequireAuth: true, QuickConnectTTL: ttl})
		},
		Remote: server.Remote, SignIn: signInTestProfile, AddViewer: addTestViewer,
		Call: jellyfinCall, Decode: decodeJellyfin, WebCall: requestWithCookie, ConfirmFactor: confirmTestFactor,
	})
}
