package mediaprobe

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"runtime"
)

var errCopiedHLSIndex = errors.New("copied HLS timeline is unavailable")

var ErrProbeLifecycle = errors.New("media probe lifecycle failed")

var ErrProbeInputUnsupported = errors.New("owned media probe input is unsupported")

type probeLoggerKey struct{}

func probeLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(probeLoggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

func ProbeLines(parent context.Context, logger *slog.Logger, executable string, arguments []string, maximumBytes int64, maximumLines int, visit func(string) error) error {
	return ProbeInputLines(parent, logger, executable, arguments, nil, maximumBytes, maximumLines, visit)
}

func ProbeInputLines(parent context.Context, logger *slog.Logger, executable string, arguments []string, input io.Reader, maximumBytes int64, maximumLines int, visit func(string) error) error {
	if input != nil && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return ErrProbeInputUnsupported
	}
	parent = context.WithValue(parent, probeLoggerKey{}, logger)
	ctx, cancel, err := copiedHLSProbeContext(parent)
	if err != nil {
		return err
	}
	defer cancel()
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdin = input
	command.Cancel = nil
	probe, output, err := startCopiedHLSProbe(ctx, command)
	if err != nil {
		if errors.Is(err, ErrProbeLifecycle) {
			reportCopiedHLSProbeCompletion(parent, nil, err, nil, nil)
		}
		return errors.Join(err, ctx.Err())
	}
	defer probe.close()
	defer output.Close()
	return readCopiedHLSProbeLines(ctx, parent, probe, output, maximumBytes, maximumLines, visit)
}

func readCopiedHLSProbeLines(ctx, parent context.Context, probe copiedHLSProbe, output io.ReadCloser, maximumBytes int64, maximumLines int, visit func(string) error) error {
	scanned := make(chan error, 1)
	watched := make(chan copiedHLSProbeWatch, 1)
	go func() { watched <- watchCopiedHLSProbe(ctx, probe, output, scanned) }()
	err := scanCopiedHLSLines(output, maximumBytes, maximumLines, visit)
	scanned <- err
	watch := <-watched
	waitErr := probe.wait()
	settleErr := settleCopiedHLSProbe(parent, probe)
	watchErr := watch.completionError(ctx, waitErr, settleErr)
	if watchErr != nil || waitErr != nil || settleErr != nil || ctx.Err() != nil {
		reportCopiedHLSProbeCompletion(ctx, err, watchErr, waitErr, settleErr)
		return errors.Join(errCopiedHLSIndex, err, ctx.Err(), watchErr)
	}
	return err
}

func scanCopiedHLSLines(output io.Reader, maximumBytes int64, maximumLines int, visit func(string) error) error {
	var err error
	scanner := bufio.NewScanner(io.LimitReader(output, maximumBytes+1))
	scanner.Buffer(make([]byte, 1024), 1024)
	var total int64
	count := 0
	for scanner.Scan() {
		total += int64(len(scanner.Bytes()) + 1)
		count++
		if total > maximumBytes || count > maximumLines {
			err = errCopiedHLSIndex
			break
		}
		if err = visit(scanner.Text()); err != nil {
			break
		}
	}
	if scanner.Err() != nil {
		err = errCopiedHLSIndex
	}
	return err
}
