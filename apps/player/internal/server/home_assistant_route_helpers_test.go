package server

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func homeAssistantRouteConfiguration(t *testing.T, data string) configuration.Snapshot {
	t.Helper()
	configured, err := configuration.Load(data, "", servertest.HomeAssistantRouteEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	return configured
}

func jellyfinRouteConfiguration(t *testing.T, data string) configuration.Snapshot {
	t.Helper()
	configured, err := configuration.Load(data, "", servertest.JellyfinRouteEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	return configured
}
