package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

type trickplay struct{ register func(*http.ServeMux) }

func newTrickplay(cache, ffmpeg string, index *libraryIndex) trickplay {
	return trickplay{register: playback.MustTrickplayRegistration(cache, ffmpeg, hlsPolicy(), index.VisibleItem, localizedError)}
}
