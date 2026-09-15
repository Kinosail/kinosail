package mediaprobe

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

const maximumProbeOutput = 2 << 20

var errProbeOutputTooLarge = errors.New("probe output is too large")

type boundedOutput struct {
	bytes.Buffer
	remaining int
	overflow  bool
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	if len(data) > output.remaining {
		written, _ := output.Buffer.Write(data[:output.remaining])
		output.remaining = 0
		output.overflow = true
		return written, errProbeOutputTooLarge
	}
	output.remaining -= len(data)
	return output.Buffer.Write(data)
}

func runProbe(ctx context.Context, executable string, arguments ...string) ([]byte, error) {
	if executable == "" || len(executable) > 4096 || strings.ContainsRune(executable, '\x00') {
		return nil, errors.New("invalid probe executable")
	}
	output := boundedOutput{remaining: maximumProbeOutput}
	//nolint:gosec // The executable is installation config. Arguments use scanned Library Content.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
