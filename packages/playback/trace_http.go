package playback

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
)

// TraceHTTPConfig binds app-owned visibility and session state to trace delivery.
type TraceHTTPConfig struct {
	Visible      func(*http.Request, string) bool
	ValidSession func(string) bool
	SetSession   func(*http.Request, string)
}

// TraceHTTP returns Player's strict playback telemetry endpoint.
func TraceHTTP(config TraceHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if config.Visible == nil || config.ValidSession == nil || config.SetSession == nil {
			apihttp.Error(writer, errors.New("playback trace unavailable"), http.StatusInternalServerError)
			return
		}
		if !config.Visible(request, request.PathValue("id")) {
			apihttp.NotFound(writer)
			return
		}
		event, err := ReadTrace(writer, request, config.ValidSession)
		if err != nil {
			apihttp.Error(writer, err, http.StatusBadRequest)
			return
		}
		config.SetSession(request, event.Session)
		logTrace(request, event)
		writer.WriteHeader(http.StatusNoContent)
	}
}

func logTrace(request *http.Request, event TraceEvent) {
	slog.InfoContext(request.Context(), "playback trace", "playback_session", event.Session, "event", event.Event, "sequence", event.Sequence, "elapsed_ms", event.ElapsedMS, "position_ms", event.PositionMS, "duration_ms", event.DurationMS, "buffered_ahead_ms", event.BufferedAheadMS, "ready_state", event.ReadyState, "network_state", event.NetworkState, "paused", event.Paused, "method", event.Method, "detail", event.Detail, "quality", event.Quality, "visibility", event.Visibility, "error_code", event.ErrorCode, "dropped_frames", event.DroppedFrames, "total_frames", event.TotalFrames)
}
