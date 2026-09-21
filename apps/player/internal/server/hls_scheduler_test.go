package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSSchedulerContract(t *testing.T) {
	servertest.HLSScheduler(t, servertest.ServerConfigAdapter[server.Config, servertest.HLSSchedulerConfig](server.New), fakePlayableHLS())
}

var firstWebItem = servertest.WebItemFactory(server.New)
