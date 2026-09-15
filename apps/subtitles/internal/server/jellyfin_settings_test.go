package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestJellyfinCompatibilityAPIIsOffByDefaultAndCanBeEnabled(t *testing.T) {
	servertest.JellyfinCompatibilityAPIIsOffByDefaultAndCanBeEnabled(t, jellyfinSettingsFixture())
}

func TestJellyfinCompatibilityWebSettingPersists(t *testing.T) {
	servertest.JellyfinCompatibilityWebSettingPersists(t, jellyfinSettingsFixture())
}

func TestJellyfinOnboardingRequiresDuckDNSBeforeEnable(t *testing.T) {
	servertest.JellyfinOnboardingRequiresDuckDNSBeforeEnable(t, jellyfinSettingsFixture())
}

func TestJellyfinWebSettingRejectsAmbiguousInputWithoutSideEffects(t *testing.T) {
	servertest.JellyfinWebSettingRejectsAmbiguousInputWithoutSideEffects(t, jellyfinSettingsFixture())
}

func jellyfinSettingsFixture() servertest.JellyfinSettingsFixture {
	return servertest.JellyfinSettingsFixture{
		Port: "38128",

		NewHandler: func(t *testing.T, dataDir, origin string, reload bool) http.Handler {
			t.Helper()
			config := server.Config{DataDir: dataDir, AuthURL: origin}
			if reload {
				configured, err := configuration.Load(dataDir, "", func(string) (string, bool) { return "", false })
				if err != nil {
					t.Fatal(err)
				}
				config.Configuration = configured
			}
			return server.New(config)
		},
		APIServer: apiServer, APICall: apiCall, JellyfinCall: jellyfinCall, WebFormCall: webFormCall,
	}
}
