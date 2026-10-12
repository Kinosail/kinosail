package mediaprobe

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"syscall"
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
		return nil, nil, parent.Err()
	}
	if deadline, ok := parent.Deadline(); ok {
		// Reserve settlement within the caller's existing deadline, exactly once.
		deadline = deadline.Add(-100 * time.Millisecond)
		if !time.Now().Before(deadline) {
			return nil, nil, context.DeadlineExceeded
		}
		ctx, cancel := context.WithDeadline(parent, deadline)
		return ctx, cancel, nil
	}
	ctx, cancel := context.WithCancel(parent)
	return ctx, cancel, nil
}

type copiedHLSProbeWatch struct {
	scanDone     bool
	stopped      bool
	scanErr      error
	result       error
	terminateErr error
	observeErr   error
}

func watchCopiedHLSProbe(ctx context.Context, probe copiedHLSProbe, output io.Closer, scanned <-chan error) copiedHLSProbeWatch {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	var watch copiedHLSProbeWatch
	for {
		if watch.observe(ctx, probe, output) {
			return watch
		}
		select {
		case watch.scanErr = <-scanned:
			watch.scanDone = true
			scanned = nil
		case <-ticker.C:
		}
	}
}

func (watch *copiedHLSProbeWatch) completionError(ctx context.Context, waitErr, settleErr error) error {
	stopErr := watch.terminateErr
	// Darwin can report EPERM for an exited, zombie-only group. Acceptance
	// still requires an active context, successful observation and Wait, and
	// proven settlement. Metadata rejection remains independent of ownership.
	if runtime.GOOS == "darwin" && errors.Is(stopErr, syscall.EPERM) && watch.lifecycleCompleted(ctx, waitErr, settleErr) {
		stopErr = nil
	}
	var exit *exec.ExitError
	unknownWait := waitErr != nil && !errors.As(waitErr, &exit) && !errors.Is(waitErr, errCopiedHLSIndex)
	if stopErr != nil || watch.observeErr != nil || settleErr != nil || unknownWait {
		return errors.Join(ErrProbeLifecycle, watch.result, stopErr, watch.observeErr, settleErr, waitErr)
	}
	return watch.result
}

func (watch *copiedHLSProbeWatch) lifecycleCompleted(ctx context.Context, waitErr, settleErr error) bool {
	return ctx.Err() == nil && watch.observeErr == nil && waitErr == nil && settleErr == nil
}

func (watch *copiedHLSProbeWatch) observe(ctx context.Context, probe copiedHLSProbe, output io.Closer) bool {
	exited, observeErr := probe.exited()
	interrupted := ctx.Err() != nil || watch.scanErr != nil || observeErr != nil
	if interrupted || exited {
		watch.stop(probe, output, interrupted)
	}
	if observeErr != nil {
		watch.observeErr = observeErr
	}
	return watch.scanDone && (exited || observeErr != nil)
}

func (watch *copiedHLSProbeWatch) stop(probe copiedHLSProbe, output io.Closer, interrupted bool) {
	if !watch.stopped {
		watch.terminateErr = probe.terminate()
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
			result = errors.Join(ErrProbeLifecycle, err)
		}
		if settled {
			break
		}
		// No ownership is released on an unproven settlement. Unix observation
		// after Wait is read-only; numeric process-group signaling has ended.
		time.Sleep(time.Millisecond)
	}
	if deadline, ok := parent.Deadline(); ok && time.Now().After(deadline) {
		probeLogger(parent).ErrorContext(parent, "copied probe settlement exceeded deadline",
			"failure_class", "copied_probe_settlement_overrun")
		return errors.Join(ErrProbeLifecycle, result)
	}
	return result
}

func reportCopiedHLSProbeCompletion(ctx context.Context, scanErr, watchErr, waitErr, settleErr error) {
	var errno syscall.Errno
	errnoKnown := errors.As(watchErr, &errno)
	exitCode, exitKnown := 0, waitErr == nil
	var exit *exec.ExitError
	if errors.As(waitErr, &exit) {
		exitCode, exitKnown = exit.ExitCode(), true
	}
	probeLogger(ctx).ErrorContext(ctx, "copied probe completion failed",
		"failure_class", "copied_probe_completion", "scan_failed", scanErr != nil,
		"watch_failed", watchErr != nil, "watch_errno", int(errno), "watch_errno_known", errnoKnown,
		"wait_failed", waitErr != nil, "wait_exit_code", exitCode, "wait_exit_known", exitKnown,
		"settlement_failed", settleErr != nil)
}
