package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLiveSearchReplacesLibraryRegion(t *testing.T) {
	t.Parallel()

	servertest.LiveSearchReplacesLibraryRegion(t, server.New(server.Config{}))
}
