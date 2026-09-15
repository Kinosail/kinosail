package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

func jellyfinBitrateTest(writer http.ResponseWriter, request *http.Request) {
	playback.JellyfinBitrateTest(writer, request)
}
