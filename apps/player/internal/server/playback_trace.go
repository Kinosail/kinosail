package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

type playbackTraceEvent = playback.TraceEvent

func apiPlaybackTrace(index *libraryIndex, preparation ...*startupPreparation) http.HandlerFunc {
	var startup *startupPreparation
	if len(preparation) > 0 {
		startup = preparation[0]
	}
	return playback.TraceHTTP(playback.TraceHTTPConfig{
		Observe: startup.observe,
		Visible: func(request *http.Request, id string) bool {
			_, found := visibleItem(request, index, id)
			return found
		},
		ValidSession: validPlaybackSession,
		SetSession:   setPlaybackSession,
	})
}
