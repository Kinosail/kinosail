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
		waitErr   error
		settleErr error
		cancel    bool
	}{
		{name: "clean", watch: copiedHLSProbeWatch{terminateErr: syscall.EPERM}},
		{name: "interrupted", watch: copiedHLSProbeWatch{result: failure, terminateErr: syscall.EPERM}},
		{name: "observation", watch: copiedHLSProbeWatch{observeErr: syscall.EPERM, terminateErr: syscall.EPERM}},
		{name: "scan", watch: copiedHLSProbeWatch{scanErr: failure, terminateErr: syscall.EPERM}},
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
			err := test.watch.completionError(ctx, test.waitErr, test.settleErr)
			settled := runtime.GOOS == "darwin" && (test.name == "clean" || test.name == "interrupted" || test.name == "scan")
			if settled {
				if !errors.Is(err, test.watch.result) || errors.Is(err, ErrProbeLifecycle) || errors.Is(err, syscall.EPERM) {
					t.Fatal("settled lifecycle hid metadata rejection or retained a zombie-only permission error")
				}
				return
			}
			assertCopiedHLSCompletion(t, err, false, test.name != "other termination")
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
		{name: "settled Darwin metadata rejection", watch: copiedHLSProbeWatch{result: errCopiedHLSIndex, terminateErr: syscall.EPERM}, scanErr: errCopiedHLSIndex, fatal: runtime.GOOS != "darwin"},
		{name: "observation failure", watch: copiedHLSProbeWatch{observeErr: failure}, fatal: true},
		{name: "termination failure", watch: copiedHLSProbeWatch{terminateErr: failure}, fatal: true},
		{name: "settlement failure", settleErr: failure, fatal: true},
		{name: "unobserved wait", waitErr: failure, fatal: true},
		{name: "ordinary nonzero exit", waitErr: &exec.ExitError{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.watch.completionError(t.Context(), test.waitErr, test.settleErr)
			if test.scanErr != nil && !errors.Is(err, test.scanErr) {
				t.Fatal("rejected metadata was accepted")
			}
			if errors.Is(err, ErrProbeLifecycle) != test.fatal {
				t.Fatal("optional metadata and uncertified lifecycle were not distinguished")
			}
		})
	}
}
