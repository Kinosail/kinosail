package server

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func duckDNSRouteConfiguration(t *testing.T, data string) configuration.Snapshot {
	t.Helper()
	configured, err := configuration.Load(data, "", servertest.JellyfinRouteEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	return configured
}
