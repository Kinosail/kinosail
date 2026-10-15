//go:build !linux && !darwin && !windows

package mediaprobe

import (
	"context"
	"io"
	"os/exec"
)

func startCopiedHLSProbe(context.Context, *exec.Cmd) (copiedHLSProbe, io.ReadCloser, error) {
	return nil, nil, errCopiedHLSIndex
}
