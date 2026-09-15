package servertest

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/playback"
)

// Real media tests are serial. This observer waits before their lifecycle cleanup.
func observeRealHLSEncoder(t *testing.T) {
	t.Helper()
	state := &realHLSEncoderState{started: make(map[string]bool)}
	restore := installRealHLSObserver(state)
	t.Cleanup(func() {
		defer restore()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 45*time.Second)
		defer cancel()
		if !playback.WaitHLSReady(ctx, state.ready) {
			t.Errorf("real HLS encoder did not complete successfully: %v", state.ready())
		}
	})
}

func installRealHLSObserver(state *realHLSEncoderState) func() {
	previous, output, flags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(realHLSEncoderObserver{Handler: previous.Handler(), state: state}))
	// The default slog handler writes through log. Preserve its original sink.
	log.SetOutput(output)
	log.SetFlags(flags)
	return func() {
		slog.SetDefault(previous)
		log.SetOutput(output)
		log.SetFlags(flags)
	}
}

type realHLSEncoderState struct {
	mu        sync.Mutex
	started   map[string]bool
	latest    string
	completed bool
	failed    bool
}

func (state *realHLSEncoderState) record(record slog.Record) {
	var request, session, mode string
	record.Attrs(func(attribute slog.Attr) bool {
		switch attribute.Key {
		case "request_id":
			request = attribute.Value.String()
		case "playback_session":
			session = attribute.Value.String()
		case "mode":
			mode = attribute.Value.String()
		}
		return true
	})
	key := request + "\x00" + session + "\x00" + mode
	state.mu.Lock()
	defer state.mu.Unlock()
	switch record.Message {
	case "HLS transcode started":
		state.started[key], state.latest, state.completed = true, key, false
	case "HLS transcode completed":
		if state.started[key] && key == state.latest {
			state.completed = true
		}
	case "HLS transcode failed":
		state.failed = state.failed || state.started[key]
	}
}

func (state *realHLSEncoderState) ready() error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failed {
		return errors.New("matching HLS encoder reported failure")
	}
	if !state.completed {
		return os.ErrNotExist
	}
	return nil
}

type realHLSEncoderObserver struct {
	slog.Handler
	state *realHLSEncoderState
}

func (observer realHLSEncoderObserver) Handle(ctx context.Context, record slog.Record) error {
	err := observer.Handler.Handle(ctx, record)
	observer.state.record(record)
	return err
}

func (observer realHLSEncoderObserver) WithAttrs(attributes []slog.Attr) slog.Handler {
	return realHLSEncoderObserver{Handler: observer.Handler.WithAttrs(attributes), state: observer.state}
}

func (observer realHLSEncoderObserver) WithGroup(name string) slog.Handler {
	return realHLSEncoderObserver{Handler: observer.Handler.WithGroup(name), state: observer.state}
}
