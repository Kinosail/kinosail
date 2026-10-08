package server

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"testing"
)

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
			if (err == nil) != accepted {
				t.Fatal("completion accepted without its complete certificate or rejected certified completion")
			}
			if !accepted && test.name != "other termination" && !errors.Is(err, syscall.EPERM) {
				t.Fatal("incomplete completion lost its native permission error")
			}
		})
	}
}
