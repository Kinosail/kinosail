package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTOTPCodesCannotBeReplayed(t *testing.T) {
	servertest.AssertTOTPCodesCannotBeReplayed(t, newJellyfinServer(t, server.Config{DataDir: t.TempDir(), RequireAuth: true}))
}
