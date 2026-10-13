package server

import (
	"context"
	"errors"
	"log/slog"
)

// Only fixed internal operation names cross the diagnostic boundary. Probe
// errors can retain sensitive paths, so the underlying error is never logged.
type transcodeClockStageError struct {
	operation string
	cause     error
}

func (failure *transcodeClockStageError) Error() string {
	return errTranscodeHLSClock.Error() + " (" + failure.operation + ")"
}
func (failure *transcodeClockStageError) Unwrap() error { return failure.cause }

func transcodeClockError(operation string, cause error) error {
	if cause == nil {
		cause = errTranscodeHLSClock
	}
	var failure *transcodeClockStageError
	if errors.As(cause, &failure) {
		return cause
	}
	return &transcodeClockStageError{operation: operation, cause: cause}
}

func reportTranscodeClockFailure(ctx context.Context, err error, budgetExpired bool) {
	operation := "completion-clock"
	var stage *transcodeClockStageError
	if errors.As(err, &stage) {
		operation = stage.operation
	}
	slog.WarnContext(ctx, "HLS transcode refill certification unavailable", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "failure_class", operation, "outcome", "ordinary-stream-retained", "budget_expired", budgetExpired)
}
