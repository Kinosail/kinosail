package server

import (
	"context"
	"errors"
	"log/slog"
)

// Only fixed internal operation names cross the diagnostic boundary. Probe
// errors can retain sensitive paths, so the underlying error is never logged.
type transcodeClockStageError struct {
	operation  string
	cause      error
	comparison []slog.Attr
}

func transcodeClockMismatch(cause error, actual *transcodeRenditionClock, original transcodeRenditionClock, number int) error {
	if cause == nil {
		cause = errTranscodeHLSClock
	}
	failure := &transcodeClockStageError{operation: "refill-stage-clock", cause: cause}
	var inner *transcodeClockStageError
	if errors.As(cause, &inner) {
		failure.operation = inner.operation
	}
	failure.comparison = []slog.Attr{slog.Bool("measured", actual != nil)}
	if actual == nil {
		return failure
	}
	failure.comparison = append(failure.comparison, slog.Bool("init_equal", actual.Initialization == original.Initialization), slog.Int("actual_cuts", len(actual.Cuts)), slog.Int64("expected_rate", original.Rate), slog.Int64("actual_rate", actual.Rate), slog.Int64("expected_base", original.VideoBase), slog.Int64("actual_base", actual.VideoBase))
	if len(actual.Cuts) == 1 {
		want, got := original.Cuts[number], actual.Cuts[0]
		failure.comparison = append(failure.comparison,
			slog.Bool("video_clock_equal", want.Clocks[0] == got.Clocks[0]), slog.Bool("audio_clock_equal", want.Clocks[1] == got.Clocks[1]),
			slog.Int64("expected_video_pts", want.VideoPTS), slog.Int64("actual_video_pts", got.VideoPTS),
			slog.Int64("expected_video_dts", want.VideoDTS), slog.Int64("actual_video_dts", got.VideoDTS),
			slog.Int64("expected_audio_pts", want.AudioPTS), slog.Int64("actual_audio_pts", got.AudioPTS),
			slog.Int("expected_video_packets", want.Packets[0]), slog.Int("actual_video_packets", got.Packets[0]),
			slog.Int("expected_audio_packets", want.Packets[1]), slog.Int("actual_audio_packets", got.Packets[1]))
	}
	return failure
}

func transcodeClockLogger(err error) *slog.Logger {
	logger := slog.Default()
	var failure *transcodeClockStageError
	if errors.As(err, &failure) && len(failure.comparison) != 0 {
		logger = logger.With(slog.GroupAttrs("clock", failure.comparison...))
	}
	return logger
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
