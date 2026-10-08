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

func watchCopiedHLSProbe(ctx context.Context, probe copiedHLSProbe, output io.Closer, scanned <-chan error) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	scanDone, stopped := false, false
	var scanErr, result error
	for {
		exited, observeErr := probe.exited()
		interrupted := ctx.Err() != nil || scanErr != nil || observeErr != nil
		if interrupted || exited {
			if !stopped {
				if probe.terminate() != nil {
					result = errCopiedHLSIndex
				}
				stopped = true
			}
			if interrupted {
				_ = output.Close()
				result = errCopiedHLSIndex
			}
		}
		if scanDone && (exited || observeErr != nil) {
			return result
		}
		select {
		case scanErr = <-scanned:
			scanDone = true
			scanned = nil
		case <-ticker.C:
		}
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
