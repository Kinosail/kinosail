package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var assetContracts = servertest.AssetsFixture{
	NewHandler: func(media, data string, auth bool) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: auth})
	},
}

func TestHomeIsInstallableAsAWebApp(t *testing.T) {
	t.Parallel()
	servertest.AssertInstallableWebApp(t, server.New(server.Config{}))
}
