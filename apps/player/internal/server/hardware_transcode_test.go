package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func hardwareTranscodeFixture() servertest.HardwareTranscodeFixture[configuration.Snapshot] {
	return servertest.HardwareTranscodeFixture[configuration.Snapshot]{
		Load: configuration.Load, SignIn: signInTestProfile, StoredState: storedState,
		New: servertest.HardwareTranscodeServer[server.Config, configuration.Snapshot](server.New),
	}
}

func TestHardwareTranscodingContract(t *testing.T) { hardwareTranscodeFixture().Run(t) }
