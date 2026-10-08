//go:build !linux && !darwin && !windows

package server

import (
	"context"
	"io"
	"os/exec"
)

func startCopiedHLSProbe(context.Context, *exec.Cmd) (copiedHLSProbe, io.ReadCloser, error) {
	// No unowned subprocess fallback on an unsupported platform.
	return nil, nil, errCopiedHLSIndex
}
