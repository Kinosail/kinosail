package server_test

import (
	"context"
	"log"
	"log/slog"
	"regexp"
	"sync"
	"testing"
)

type remainingNonKeyLog struct {
	mu       sync.Mutex
	failures map[string]string
	overflow bool
}

type remainingNonKeyLogHandler struct {
	slog.Handler
	observed *remainingNonKeyLog
}

// Public queued does not preserve the internal preparation result. Observe only
// its bounded request identity/state/phase; no raw media or process log is saved.
func remainingNonKeyObserve(t *testing.T) *remainingNonKeyLog {
	t.Helper()
	previous := slog.Default()
	output, flags := log.Writer(), log.Flags()
	observed := &remainingNonKeyLog{failures: make(map[string]string)}
	slog.SetDefault(slog.New(remainingNonKeyLogHandler{Handler: slog.DiscardHandler, observed: observed}))
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(output)
		log.SetFlags(flags)
	})
	return observed
}

func (remainingNonKeyLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (handler remainingNonKeyLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message != "HLS startup preparation" {
		return nil
	}
	requestID, state, phase := "", "", ""
	record.Attrs(func(attribute slog.Attr) bool {
		switch attribute.Key {
		case "request_id":
			requestID = attribute.Value.String()
		case "state":
			state = attribute.Value.String()
		case "phase":
			phase = attribute.Value.String()
		}
		return true
	})
	if state == "unavailable" && remainingNonKeyValidRequestID(requestID) && len(phase) > 0 && len(phase) <= 64 {
		handler.observed.mu.Lock()
		if len(handler.observed.failures) >= 16 {
			handler.observed.overflow = true
		} else {
			handler.observed.failures[requestID] = phase
		}
		handler.observed.mu.Unlock()
	}
	return handler.Handler.Handle(ctx, record)
}

func (handler remainingNonKeyLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return remainingNonKeyLogHandler{Handler: handler.Handler.WithAttrs(attrs), observed: handler.observed}
}

func (handler remainingNonKeyLogHandler) WithGroup(name string) slog.Handler {
	return remainingNonKeyLogHandler{Handler: handler.Handler.WithGroup(name), observed: handler.observed}
}

func (observed *remainingNonKeyLog) rejection(requestID string) (string, bool) {
	observed.mu.Lock()
	defer observed.mu.Unlock()
	if observed.overflow {
		return "observation-bound", true
	}
	phase, found := observed.failures[requestID]
	return phase, found
}

var remainingNonKeyRequestIdentity = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func remainingNonKeyValidRequestID(value string) bool {
	return remainingNonKeyRequestIdentity.MatchString(value)
}
