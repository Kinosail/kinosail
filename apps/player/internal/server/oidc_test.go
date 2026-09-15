package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOIDCContract(t *testing.T) {
	servertest.OIDCContract(t, oidcFixture())
}

func oidcFixture() servertest.OIDCFixture {
	return servertest.OIDCFixture{
		NewHandler: func(dataDir string, oidc server.OIDCConfig, scim server.SCIMConfig) http.Handler {
			return server.New(server.Config{DataDir: dataDir, RequireAuth: true, OIDC: oidc, SCIM: scim})
		},
		StoredState: storedState, TOTP: testTOTP,
	}
}
