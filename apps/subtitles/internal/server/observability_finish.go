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

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

type requestObservation struct {
	activity *auditStore
	route    func(*http.Request) string
	request  *http.Request
	facts    *requestActivity
	capture  *auditWriter
	started  time.Time
}

func (observation requestObservation) finish(ctx context.Context) {
	panicked := observation.recover(recover())
	status := observation.status()
	observation.record(status, panicked)
	pattern := observation.route(observation.request)
	if level := requestLogLevel(observation.request, observation.facts, pattern, status); slog.Default().Enabled(ctx, level) {
		slog.Log(ctx, level, "request", observation.attributes(status, pattern)...)
	}
}

func (observation requestObservation) recover(recovered any) bool {
	if recovered == nil {
		return false
	}
	if observation.capture.Status == 0 {
		http.Error(observation.capture, "internal server error", http.StatusInternalServerError)
	}
	slog.Error("request panic", "request_id", httpguard.LogText(observation.facts.id, 64), "method", httpguard.LogMethod(observation.request.Method), "route", observation.route(observation.request), "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
	return true
}

func (observation requestObservation) status() int {
	if observation.capture.Status == 0 {
		return http.StatusOK
	}
	return observation.capture.Status
}

func (observation requestObservation) record(status int, panicked bool) {
	observation.activity.Observe(observation.facts.id, observation.request.Method, observation.request.URL.Path, status, time.Since(observation.started), panicked)
	if (status == http.StatusUnauthorized || status == http.StatusForbidden) && !observation.facts.securityRecorded {
		observation.activity.Denied(observation.request, http.StatusText(status))
	}
}

func requestLogLevel(request *http.Request, facts *requestActivity, pattern string, status int) slog.Level {
	level := slog.LevelDebug
	if request.Method != http.MethodGet && request.Method != http.MethodHead || status >= http.StatusBadRequest || facts.playbackSession != "" && !strings.HasPrefix(pattern, "GET /hls/") {
		level = slog.LevelInfo
	}
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	} else if status >= http.StatusBadRequest {
		level = slog.LevelWarn
	}
	if facts.logLimit != nil && level > *facts.logLimit {
		return *facts.logLimit
	}
	return level
}

func (observation requestObservation) attributes(status int, pattern string) []any {
	attributes := []any{"request_id", httpguard.LogText(observation.facts.id, 64), "method", httpguard.LogMethod(observation.request.Method), "route", pattern, "status", status, "duration_ms", time.Since(observation.started).Milliseconds()}
	if path := identitycore.SafeCompatibilityPath(observation.request, pattern); path != "" {
		attributes = append(attributes, "compatibility_path", httpguard.LogText(path, 512))
	}
	if observation.facts.playbackSession == "" {
		return attributes
	}
	attributes = append(attributes, "playback_session", httpguard.LogText(observation.facts.playbackSession, 64), "response_bytes", observation.capture.Bytes)
	if value := observation.request.Header.Get("Range"); identitycore.ValidSingleByteRange(value) {
		attributes = append(attributes, "request_range", httpguard.LogText(value, 128))
	}
	if value := observation.capture.Header().Get("Content-Range"); identitycore.ValidContentRange(value) {
		attributes = append(attributes, "response_range", httpguard.LogText(value, 128))
	}
	if value, _, err := mime.ParseMediaType(observation.capture.Header().Get("Content-Type")); err == nil {
		attributes = append(attributes, "response_type", httpguard.LogText(value, 128))
	}
	return attributes
}
