package servertest

import (
	"bytes"
	"errors"
	"log"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestRealHLSEncoderObserverPreservesDefaultLoggingBridge(t *testing.T) {
	state := &realHLSEncoderState{started: make(map[string]bool)}
	previous, output, flags := slog.Default(), log.Writer(), log.Flags()
	restore := installRealHLSObserver(state)
	t.Cleanup(restore)
	if log.Writer() != output || log.Flags() != flags {
		t.Fatal("observer redirected the default handler back into itself")
	}
	slog.Info("HLS transcode started", "request_id", "bridge-test")
	slog.Info("HLS transcode completed", "request_id", "bridge-test")
	if err := state.ready(); err != nil {
		t.Fatal(err)
	}
	restore()
	if slog.Default() != previous || log.Writer() != output || log.Flags() != flags {
		t.Fatal("observer did not restore logging configuration")
	}
}

func TestRealHLSEncoderObserverRequiresMatchingLatestCompletion(t *testing.T) {
	state := &realHLSEncoderState{started: make(map[string]bool)}
	var output bytes.Buffer
	logger := slog.New(realHLSEncoderObserver{Handler: slog.NewJSONHandler(&output, nil), state: state})
	logger.Info("HLS transcode completed", "request_id", "unrelated")
	if !errors.Is(state.ready(), os.ErrNotExist) {
		t.Fatal("unrelated completion satisfied the encoder wait")
	}
	logger.Info("HLS transcode started", "request_id", "original")
	logger.Info("HLS transcode started", "request_id", "replacement")
	logger.Info("HLS transcode completed", "request_id", "original")
	if !errors.Is(state.ready(), os.ErrNotExist) {
		t.Fatal("superseded completion satisfied the current encoder wait")
	}
	logger.Info("HLS transcode completed", "request_id", "replacement")
	if err := state.ready(); err != nil {
		t.Fatalf("matching encoder completion was ignored: %v", err)
	}
	if strings.Count(output.String(), "\n") != 5 || !strings.Contains(output.String(), `"request_id":"unrelated"`) {
		t.Fatal("observer did not forward every log record")
	}
}

func TestRealHLSEncoderObserverDoesNotHideMatchingFailure(t *testing.T) {
	state := &realHLSEncoderState{started: make(map[string]bool)}
	var output bytes.Buffer
	logger := slog.New(realHLSEncoderObserver{Handler: slog.NewJSONHandler(&output, nil), state: state})
	logger.Info("HLS transcode started", "request_id", "owned")
	logger.Error("HLS transcode failed", "request_id", "other")
	if !errors.Is(state.ready(), os.ErrNotExist) {
		t.Fatal("unrelated failure changed the pending encoder state")
	}
	logger.Error("HLS transcode failed", "request_id", "owned")
	logger.Info("HLS transcode completed", "request_id", "owned")
	if err := state.ready(); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("matching failure was hidden by a later completion: %v", err)
	}
	if strings.Count(output.String(), `"level":"ERROR"`) != 2 {
		t.Fatal("observer hid encoder error records")
	}
}
