package playback

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

var errInvalidTrickplayRegistration = errors.New("trickplay dependencies are invalid")

func MustTrickplayRegistration(
	cache, ffmpeg string,
	policy HLSRecipePolicy,
	lookup func(*http.Request, string) (library.Item, bool),
	failure func(http.ResponseWriter, *http.Request, string, int),
) func(*http.ServeMux) {
	if lookup == nil || failure == nil {
		panic(errInvalidTrickplayRegistration)
	}
	frames, err := NewTrickplay(TrickplayDependencies{
		Cache: cache, FFmpeg: ffmpeg, RecipePolicy: policy, Lookup: lookup,
		NotFound: func(writer http.ResponseWriter, request *http.Request) {
			failure(writer, request, "not found", http.StatusNotFound)
		},
		Unavailable: func(writer http.ResponseWriter, request *http.Request) {
			failure(writer, request, "preview unavailable", http.StatusServiceUnavailable)
		},
	})
	if err != nil {
		panic(err)
	}
	return frames.Register
}
