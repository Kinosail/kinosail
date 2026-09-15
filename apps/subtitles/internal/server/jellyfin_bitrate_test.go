package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestJellyfinBitrateContract(t *testing.T) {
	servertest.JellyfinBitrate(t, apiServer, enableJellyfin, jellyfinCall)
}
