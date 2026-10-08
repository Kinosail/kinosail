//go:build !linux && !darwin && !windows

package server

import (
	"context"
	"io"
)

func startCopiedHLSProbe(context.Context, string, []string) (copiedHLSProbe, io.ReadCloser, error) {
	// No unowned subprocess fallback on an unsupported platform.
	return nil, nil, errCopiedHLSIndex
}
