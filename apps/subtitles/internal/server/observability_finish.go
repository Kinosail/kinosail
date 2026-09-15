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
	attributes := observation.attributes(status, pattern)
	slog.Log(ctx, requestLogLevel(observation.request, observation.facts, pattern, status), "request", attributes...)
}

func (observation requestObservation) recover(recovered any) bool {
	if recovered == nil {
		return false
	}
	if observation.capture.Status == 0 {
		http.Error(observation.capture, "internal server error", http.StatusInternalServerError)
	}
	slog.Error("request panic", "request_id", observation.facts.id, "method", observation.request.Method, "route", observation.route(observation.request), "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
	return true
}

func (observation requestObservation) status() int {
	if observation.capture.Status == 0 {
		return http.StatusOK
	}
	return observation.capture.Status
}

func (observation requestObservation) record(status int, panicked bool) {
	observation.activity.requests.Add(1)
	if status >= http.StatusBadRequest {
		observation.activity.requestErrors.Add(1)
	}
	if panicked {
		observation.activity.panics.Add(1)
	}
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
		return slog.LevelError
	}
	if status >= http.StatusBadRequest {
		return slog.LevelWarn
	}
	return level
}

func (observation requestObservation) attributes(status int, pattern string) []any {
	attributes := []any{"request_id", observation.facts.id, "method", observation.request.Method, "route", pattern, "status", status, "duration_ms", time.Since(observation.started).Milliseconds()}
	if path := identitycore.SafeCompatibilityPath(observation.request, pattern); path != "" {
		attributes = append(attributes, "compatibility_path", path)
	}
	if observation.facts.playbackSession == "" {
		return attributes
	}
	attributes = append(attributes, "playback_session", observation.facts.playbackSession, "response_bytes", observation.capture.Bytes)
	if value := observation.request.Header.Get("Range"); mediashares.ValidSingleByteRange(value) {
		attributes = append(attributes, "request_range", value)
	}
	if value := observation.capture.Header().Get("Content-Range"); identitycore.ValidContentRange(value) {
		attributes = append(attributes, "response_range", value)
	}
	if value, _, err := mime.ParseMediaType(observation.capture.Header().Get("Content-Type")); err == nil {
		attributes = append(attributes, "response_type", value)
	}
	return attributes
}
