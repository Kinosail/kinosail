package mediaprobe

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

// Real-media E2E cannot reliably cause OS observation or termination failures.
// This retained certificate seam verifies that optional probing cannot hide them.
func TestCopiedHLSLinesCompletionCertificate(t *testing.T) {
	failure := errors.New("incomplete probe")
	cases := []struct {
		name      string
		watch     copiedHLSProbeWatch
		scanErr   error
		waitErr   error
		settleErr error
		cancel    bool
	}{
		{name: "clean", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}},
		{name: "interrupted", watch: copiedHLSProbeWatch{result: failure, terminateErr: syscall.EPERM}},
		{name: "observation", watch: copiedHLSProbeWatch{observeErr: syscall.EPERM, terminateErr: syscall.EPERM}},
		{name: "scan", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}, scanErr: failure},
		{name: "wait", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}, waitErr: failure},
		{name: "settlement", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}, settleErr: failure},
		{name: "cancelled", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}, cancel: true},
		{name: "other termination", watch: copiedHLSProbeWatch{terminateErr: failure}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.cancel {
				cancel()
			}
			err := test.watch.completionError(ctx, test.scanErr, test.waitErr, test.settleErr)
			accepted := runtime.GOOS == "darwin" && test.name == "clean"
			assertCopiedHLSCompletion(t, err, accepted, test.name != "other termination")
		})
	}
}

func assertCopiedHLSCompletion(t *testing.T, err error, accepted, permissionError bool) {
	t.Helper()
	if (err == nil) != accepted {
		t.Fatal("completion accepted without its complete certificate or rejected certified completion")
	}
	if accepted {
		return
	}
	if permissionError && !errors.Is(err, syscall.EPERM) {
		t.Fatal("incomplete completion lost its native permission error")
	}
	if !errors.Is(err, ErrProbeLifecycle) {
		t.Fatal("uncertified lifecycle was classified as optional metadata failure")
	}
}

func TestCopiedHLSLinesFailureClasses(t *testing.T) {
	failure := errors.New("probe observation failed")
	cases := []struct {
		name      string
		watch     copiedHLSProbeWatch
		scanErr   error
		settleErr error
		waitErr   error
		fatal     bool
	}{
		{name: "unsupported metadata", watch: copiedHLSProbeWatch{result: errCopiedHLSIndex}, scanErr: errCopiedHLSIndex},
		{name: "observation failure", watch: copiedHLSProbeWatch{observeErr: failure}, fatal: true},
		{name: "termination failure", watch: copiedHLSProbeWatch{terminateErr: failure}, fatal: true},
		{name: "settlement failure", settleErr: failure, fatal: true},
		{name: "unobserved wait", waitErr: failure, fatal: true},
		{name: "ordinary nonzero exit", waitErr: &exec.ExitError{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.watch.completionError(t.Context(), test.scanErr, test.waitErr, test.settleErr)
			if errors.Is(err, ErrProbeLifecycle) != test.fatal {
				t.Fatal("optional metadata and uncertified lifecycle were not distinguished")
			}
		})
	}
}
