package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestJellyfinDownloadResumesConditionallyAcrossReplacementAndRestart(t *testing.T) {
	fixture := servertest.JellyfinDownloadFixture{SignIn: signInTestProfile, Login: jellyfinLogin, Call: jellyfinCall, New: func(t *testing.T, media, data, cache string, restart bool) http.Handler {
		t.Helper()
		config := server.Config{MediaDir: media, DataDir: data, CacheDir: cache, RequireAuth: true}
		if restart {
			return server.New(trustedJellyfinConfig(t, config))
		}
		return newJellyfinServer(t, config)
	}}
	fixture.ResumesConditionallyAcrossReplacementAndRestart(t)
}
