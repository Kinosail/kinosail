package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAgentConnectionsExposeAccessibleConsentAndSettingsLandmarks(t *testing.T) {
	t.Parallel()
	origin := "https://kino.test:38128"
	servertest.AgentConnectionsExposeAccessibleConsentAndSettingsLandmarks(t, server.New(server.Config{DataDir: t.TempDir(), AuthURL: origin}), origin)
}
