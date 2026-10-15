package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	servertest.WriteExecutable(t, path, content)
}

func fakePlayableHLS() string { return servertest.SingleSegmentPlayableHLS() }
