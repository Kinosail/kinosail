package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOIDCContract(t *testing.T) {
	servertest.OIDCContract(t, oidcFixture())
}

func TestUnconfiguredSSOLogsWarnAndProviderOutageLogsError(t *testing.T) {
	servertest.AssertUnconfiguredSSOLogsWarnAndProviderOutageLogsError(t, oidcFixture())
}

func oidcFixture() servertest.OIDCFixture {
	return servertest.OIDCFixture{
		NewHandler: func(dataDir string, oidc server.OIDCConfig, _ scim.Config) http.Handler {
			return server.New(server.Config{DataDir: dataDir, RequireAuth: true, OIDC: oidc})
		},
		StoredState: storedState, TOTP: testTOTP, NoSCIM: true,
	}
}
