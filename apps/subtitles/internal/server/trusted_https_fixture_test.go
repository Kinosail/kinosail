package server_test

import (
	"net/http"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func trustedHTTPSFixture() servertest.TrustedHTTPSFixture {
	fixture := servertest.TrustedHTTPSFixture{Origin: configuration.TrustedOrigin, SignIn: signInTestProfile, Web: requestWithCookie, Token: trustedHTTPSTestToken, ExpectedOrigin: "https://family-media.duckdns.org:38128"}
	return servertest.BindTrustedHTTPS(fixture, configuration.Load, func(directory string, configured configuration.Snapshot) http.Handler {
		return server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	})
}
