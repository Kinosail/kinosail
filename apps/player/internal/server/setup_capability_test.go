package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSetupCapabilityContract(t *testing.T) {
	servertest.SetupCapability(t, func(dataDir string) http.Handler {
		return server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	})
}
