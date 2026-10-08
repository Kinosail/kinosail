package server

import (
	"context"
	"io"
	"log/slog"
	"time"
)

type copiedHLSProbe interface {
	exited() (bool, error)
	terminate() error
	wait() error
	settled() (bool, error)
	close()
}

func copiedHLSProbeContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if parent.Err() != nil {
		return nil, nil, errCopiedHLSIndex
	}
	if deadline, ok := parent.Deadline(); ok {
		// Reserve settlement within the caller's existing deadline, exactly once.
		deadline = deadline.Add(-100 * time.Millisecond)
		if !time.Now().Before(deadline) {
			return nil, nil, errCopiedHLSIndex
		}
		ctx, cancel := context.WithDeadline(parent, deadline)
		return ctx, cancel, nil
	}
	ctx, cancel := context.WithCancel(parent)
	return ctx, cancel, nil
}

type copiedHLSProbeWatch struct {
	scanDone bool
	stopped  bool
	scanErr  error
	result   error
}

func watchCopiedHLSProbe(ctx context.Context, probe copiedHLSProbe, output io.Closer, scanned <-chan error) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	var watch copiedHLSProbeWatch
	for {
		if watch.observe(ctx, probe, output) {
			return watch.result
		}
		select {
		case watch.scanErr = <-scanned:
			watch.scanDone = true
			scanned = nil
		case <-ticker.C:
		}
	}
}

func (watch *copiedHLSProbeWatch) observe(ctx context.Context, probe copiedHLSProbe, output io.Closer) bool {
	exited, observeErr := probe.exited()
	interrupted := ctx.Err() != nil || watch.scanErr != nil || observeErr != nil
	if interrupted || exited {
		watch.stop(probe, output, interrupted)
	}
	return watch.scanDone && (exited || observeErr != nil)
}

func (watch *copiedHLSProbeWatch) stop(probe copiedHLSProbe, output io.Closer, interrupted bool) {
	if !watch.stopped {
		if probe.terminate() != nil {
			watch.result = errCopiedHLSIndex
		}
		watch.stopped = true
	}
	if interrupted {
		_ = output.Close()
		watch.result = errCopiedHLSIndex
	}
}

func settleCopiedHLSProbe(parent context.Context, probe copiedHLSProbe) error {
	var result error
	for {
		settled, err := probe.settled()
		if err != nil {
			result = errCopiedHLSIndex
		}
		if settled {
			break
		}
		// No ownership is released on an unproven settlement. Unix observation
		// after Wait is read-only; numeric process-group signaling has ended.
		time.Sleep(time.Millisecond)
	}
	if deadline, ok := parent.Deadline(); ok && time.Now().After(deadline) {
		slog.ErrorContext(parent, "copied probe settlement exceeded deadline",
			"request_id", requestActivityID(parent), "playback_session", requestPlaybackSession(parent),
			"failure_class", "copied_probe_settlement_overrun")
		return errCopiedHLSIndex
	}
	return result
}
