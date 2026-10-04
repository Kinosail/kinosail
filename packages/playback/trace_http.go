package playback

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/apihttp"
)

// TraceHTTPConfig binds app-owned visibility and session state to trace delivery.
type TraceHTTPConfig struct {
	Observe      func(*http.Request, TraceEvent)
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
		if config.Observe != nil {
			config.Observe(request, event)
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func logTrace(request *http.Request, event TraceEvent) {
	level := slog.LevelInfo
	if event.Event == "error" {
		level = slog.LevelWarn
	}
	text := func(value string) string { return strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "") }
	slog.LogAttrs(request.Context(), level, "playback trace",
		slog.String("playback_session", text(event.Session)), slog.String("event", text(event.Event)),
		slog.Int64("sequence", event.Sequence), slog.Int64("elapsed_ms", event.ElapsedMS),
		slog.Int64("position_ms", event.PositionMS), slog.Int64("duration_ms", event.DurationMS),
		slog.Int64("buffered_ahead_ms", event.BufferedAheadMS), slog.Int("ready_state", event.ReadyState),
		slog.Int("network_state", event.NetworkState), slog.Bool("paused", event.Paused),
		slog.String("method", text(event.Method)), slog.String("detail", text(safeTraceDetail(event))),
		slog.String("visibility", text(event.Visibility)),
		slog.Int("error_code", event.ErrorCode), slog.Int64("dropped_frames", event.DroppedFrames),
		slog.Int64("total_frames", event.TotalFrames))
}

func safeTraceDetail(event TraceEvent) string {
	// Retain named failures from known controls; arbitrary diagnostic text can contain credentials.
	detail := ""
	parts := strings.Split(event.Detail, ":")
	if event.Event == "error" && len(parts) == 3 && parts[0] == "fullscreen" && parts[2] == "playback-retained" && oneOf(parts[1], "NotAllowedError", "InvalidStateError", "NotSupportedError", "TypeError", "Error") {
		detail = event.Detail
	}
	if event.Event == "play-rejected" && len(parts) == 2 && oneOf(parts[0], "control", "apple-play", "autoplay-canplay", "keyboard", "media-element", "watch-room", "media-session", "queue-advance", "resume-progress", "home-assistant", "source-change", "offline-source") && oneOf(parts[1], "NotAllowedError", "NotSupportedError", "AbortError", "InvalidStateError", "TypeError", "Error") {
		detail = event.Detail
	}
	return detail
}
