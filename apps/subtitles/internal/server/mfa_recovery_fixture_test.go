package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func newMFARecoveryServer(t *testing.T, data string) http.Handler {
	t.Helper()
	return newTrustedServer(t, server.Config{DataDir: data, RequireAuth: true})
}

func restartMFARecoveryServer(t *testing.T, data string) http.Handler {
	t.Helper()
	return server.New(trustedHTTPSConfig(t, server.Config{DataDir: data, RequireAuth: true}))
}
