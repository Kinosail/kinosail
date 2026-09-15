package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

type playbackTraceEvent = playback.TraceEvent

func apiPlaybackTrace(index *libraryIndex) http.HandlerFunc {
	return playback.TraceHTTP(playback.TraceHTTPConfig{
		Visible: func(request *http.Request, id string) bool {
			_, found := visibleItem(request, index, id)
			return found
		},
		ValidSession: validPlaybackSession,
		SetSession:   setPlaybackSession,
	})
}
