package server_test

import (
	"net/http"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var libraryAPIFixture = servertest.LibraryAPIFixture{
	NewHandler: func(media, data string, requireAuth bool) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: requireAuth})
	},
	SignIn:          signInTestProfile,
	Server:          apiServer,
	StoredState:     storedState,
	StoredProfileID: storedProfileID,
}

var paginatedLibrary = libraryAPIFixture.PaginatedLibrary
