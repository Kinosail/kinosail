package server

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

type requestActivityKey struct{}

type requestActivity struct {
	id               string
	playbackSession  string
	viewer           viewerProfile
	securityRecorded bool
}

func observeRequests(activity *auditStore, route func(*http.Request) string, next http.Handler) http.Handler { //nolint:contextcheck,cyclop,gocognit // Recovery and final logging intentionally retain the request context.
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		facts := &requestActivity{id: acceptedRequestID(request.Header.Get("X-Request-ID")), playbackSession: acceptedPlaybackSession(request)}
		request = request.WithContext(context.WithValue(request.Context(), requestActivityKey{}, facts))
		writer.Header().Set("X-Request-ID", facts.id)
		capture := &auditWriter{ResponseWriter: writer}
		started := time.Now()
		defer func() { //nolint:contextcheck // Deferred recovery must report against the captured request context.
			finishObservedRequest(activity, route, capture, request, facts, started, recover())
		}()
		next.ServeHTTP(capture, request)
	})
}

func finishObservedRequest(activity *auditStore, route func(*http.Request) string, capture *auditWriter, request *http.Request, facts *requestActivity, started time.Time, recovered any) {
	panicked := recoverObservedRequest(capture, request, facts, route, recovered)
	status := capture.Status
	if status == 0 {
		status = http.StatusOK
	}
	recordObservedStatus(activity, request, facts, status, panicked)
	activity.failures.Record(facts.id, request.Method, request.URL.Path, status, time.Since(started))
	pattern := route(request)
	if jellyfinMediaPath(request.URL.Path) && status >= http.StatusBadRequest {
		slog.Warn("Jellyfin media authentication diagnostic", "diagnostic", "[PLAYBACK-AUTH]", "request_id", facts.id, "route", pattern, "status", status, "token_source", sessionTokenSource(request), "api_key_query", jellyfinQueryShape(request, "api_key"), "play_session_query", jellyfinQueryShape(request, "playSessionId"), "client", jellyfinClientClass(request))
	}
	slog.Log(request.Context(), observedRequestLevel(request, pattern, status, facts), "request", observedRequestAttributes(capture, request, facts, pattern, status, started)...)
}

func recoverObservedRequest(capture *auditWriter, request *http.Request, facts *requestActivity, route func(*http.Request) string, recovered any) bool {
	if recovered == nil {
		return false
	}
	if capture.Status == 0 {
		http.Error(capture, "internal server error", http.StatusInternalServerError)
	}
	slog.Error("request panic", "request_id", facts.id, "method", request.Method, "route", route(request), "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
	return true
}

func recordObservedStatus(activity *auditStore, request *http.Request, facts *requestActivity, status int, panicked bool) {
	activity.requests.Add(1)
	if status >= http.StatusBadRequest {
		activity.requestErrors.Add(1)
	}
	if panicked {
		activity.panics.Add(1)
	}
	if (status == http.StatusUnauthorized || status == http.StatusForbidden) && !facts.securityRecorded {
		activity.Denied(request, http.StatusText(status))
	}
}

func observedRequestLevel(request *http.Request, pattern string, status int, facts *requestActivity) slog.Level {
	level := slog.LevelDebug
	if request.Method != http.MethodGet && request.Method != http.MethodHead || status >= http.StatusBadRequest || facts.playbackSession != "" && !strings.HasPrefix(pattern, "GET /hls/") && !jellyfinMediaPath(request.URL.Path) {
		level = slog.LevelInfo
	}
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if status >= http.StatusBadRequest {
		return slog.LevelWarn
	}
	return level
}

func observedRequestAttributes(capture *auditWriter, request *http.Request, facts *requestActivity, pattern string, status int, started time.Time) []any {
	attributes := []any{"request_id", facts.id, "method", request.Method, "route", pattern, "status", status, "duration_ms", time.Since(started).Milliseconds()}
	if jellyfinMediaPath(request.URL.Path) {
		attributes = append(attributes, "stream_shape", jellyfinStreamShape(request.URL.Path), "client", jellyfinClientClass(request))
	}
	if path := identitycore.SafeCompatibilityPath(request, pattern); path != "" {
		attributes = append(attributes, "compatibility_path", path)
	}
	if facts.playbackSession == "" {
		return attributes
	}
	attributes = append(attributes, "playback_session", facts.playbackSession, "response_bytes", capture.Bytes)
	if value := request.Header.Get("Range"); mediashares.ValidSingleByteRange(value) {
		attributes = append(attributes, "request_range", value)
	}
	if value := capture.Header().Get("Content-Range"); identitycore.ValidContentRange(value) {
		attributes = append(attributes, "response_range", value)
	}
	if value, _, err := mime.ParseMediaType(capture.Header().Get("Content-Type")); err == nil {
		attributes = append(attributes, "response_type", value)
	}
	return attributes
}

func jellyfinQueryShape(request *http.Request, name string) string {
	values, present := request.URL.Query()[name]
	if !present {
		return "absent"
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > 256 || name == "playSessionId" && !validPlaybackSession(values[0]) {
		return "invalid"
	}
	return "present"
}

func jellyfinClientClass(request *http.Request) string {
	userAgent := strings.ToLower(request.UserAgent())
	switch {
	case strings.Contains(userAgent, "swiftfin"):
		return "swiftfin"
	case strings.Contains(userAgent, "tvos") || strings.Contains(userAgent, "appletv") || strings.Contains(userAgent, "apple tv"):
		return "apple-tv"
	case strings.Contains(userAgent, "jellyfin"):
		return "jellyfin"
	case userAgent == "":
		return "absent"
	default:
		return "other"
	}
}

func jellyfinMediaPathShape(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "Videos" && parts[0] != "Audio" {
		return "invalid"
	}
	if len(parts) == 3 {
		return "item/stream"
	}
	return "item/source/stream"
}

func jellyfinStreamShape(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return "invalid"
	}
	stream := strings.ToLower(strings.Join(parts[2:], "/"))
	last := stream[strings.LastIndexByte(stream, '/')+1:]
	if strings.Contains(stream, "/subtitles/") {
		return "subtitle"
	}
	if strings.HasPrefix(strings.ToLower(last), "stream") {
		return "direct"
	}
	if strings.EqualFold(last, "master.m3u8") {
		return "master"
	}
	if _, file, planned := plannedHLSFile(stream); planned {
		return "planned-" + hlsFileShape(file)
	}
	if hlsFile(stream) {
		return "session-" + hlsFileShape(stream)
	}
	if file, sourceHLS := jellyfinSourceHLSFile(stream); sourceHLS {
		return "source-" + hlsFileShape(file)
	}
	return "other"
}

func hlsFileShape(name string) string {
	last := name[strings.LastIndexByte(name, '/')+1:]
	switch {
	case name == "index.m3u8":
		return "master"
	case last == "index.m3u8":
		return "variant"
	case last == "init.mp4":
		return "init"
	case strings.HasPrefix(last, "segment-"):
		return "segment"
	default:
		return "other"
	}
}
