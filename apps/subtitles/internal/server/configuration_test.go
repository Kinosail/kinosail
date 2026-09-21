package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestConfigurationHTTPContract(t *testing.T) {
	fixture := servertest.ConfigurationHTTP[configuration.Source, configuration.Snapshot]{
		Load: configuration.Load, GUI: configuration.GUI, SCIMToken: scimTestToken,
		SignIn: signInTestProfile, WebCall: requestWithCookie,
		TMDBSection: `href="/settings/configuration#integrations.tmdb.token"`, TMDBLabel: "TMDB access token", TMDBHelp: "Environment variable: <code>KINOSAIL_TMDB_TOKEN</code>",
		NewHandler: func(media, directory string, configured configuration.Snapshot) http.Handler {
			return server.New(server.Config{MediaDir: media, DataDir: directory, RequireAuth: true, Configuration: configured})
		},
	}
	fixture.Run(t)
}
