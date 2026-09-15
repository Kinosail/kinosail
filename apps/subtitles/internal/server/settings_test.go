package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSettingsRegression(t *testing.T) { servertest.RunSettings(t, settingsRegression()) }
func settingsRegression() servertest.SettingsRegression {
	return servertest.SettingsRegression{
		New: func(f servertest.SettingsFixture) http.Handler {
			return server.New(server.Config{MediaDir: f.MediaDir, DataDir: f.DataDir, CacheDir: f.CacheDir, FFmpeg: f.FFmpeg})
		},
		PlayableHLS: fakePlayableHLS, APICall: apiCall, AssertBody: assertAPIBody,
		ProtectionText: "Protected automatically",
	}
}
