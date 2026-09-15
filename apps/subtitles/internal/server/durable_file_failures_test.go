package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestInvalidDurableShapesFailApplicationClosed(t *testing.T) {
	libraryAPIFixture.InvalidDurableShapesFailClosed(t)
}

func TestInvalidDownloadStateFailsApplicationClosed(t *testing.T) {
	servertest.InvalidDownloadStateFailsClosed(t, func(data, cache string) http.Handler {
		return server.New(server.Config{DataDir: data, CacheDir: cache})
	})
}

func TestVersionedAPISurfacesIndividualStateFileFailures(t *testing.T) {
	libraryAPIFixture.IndividualStateFileFailures(t, firstAPIItemID)
}
