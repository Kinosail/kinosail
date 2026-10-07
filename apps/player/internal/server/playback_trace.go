package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

type playbackTraceEvent = playback.TraceEvent

func apiPlaybackTrace(index *libraryIndex, hls *hlsManager) http.HandlerFunc {
	return playback.TraceHTTP(playback.TraceHTTPConfig{
		Observe: func(request *http.Request, event playback.TraceEvent) {
			hls.startup.observe(request, event)
			if event.Event == "session-end" {
				hls.stopHLSPage(request)
			}
		},
		Visible: func(request *http.Request, id string) bool {
			_, found := visibleItem(request, index, id)
			return found
		},
		ValidSession: validPlaybackSession,
		SetSession:   setPlaybackSession,
	})
}
