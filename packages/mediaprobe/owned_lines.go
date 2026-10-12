package mediaprobe

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
)

var errCopiedHLSIndex = errors.New("copied HLS timeline is unavailable")

type probeLoggerKey struct{}

func probeLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(probeLoggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// ProbeLines reads bounded metadata and joins the complete configured process tree.
// The caller supplies a deadline and a logger with already-validated correlation fields.
func ProbeLines(parent context.Context, logger *slog.Logger, executable string, arguments []string, maximumBytes int64, maximumLines int, visit func(string) error) error {
	parent = context.WithValue(parent, probeLoggerKey{}, logger)
	ctx, cancel, err := copiedHLSProbeContext(parent)
	if err != nil {
		return err
	}
	defer cancel()
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Cancel = nil // The joined owner below settles the entire process group.
	probe, output, err := startCopiedHLSProbe(ctx, command)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer probe.close()
	defer output.Close()
	scanned := make(chan error, 1)
	watched := make(chan copiedHLSProbeWatch, 1)
	go func() { watched <- watchCopiedHLSProbe(ctx, probe, output, scanned) }()
	err = scanCopiedHLSLines(output, maximumBytes, maximumLines, visit)
	scanned <- err
	watch := <-watched
	waitErr := probe.wait()
	settleErr := settleCopiedHLSProbe(parent, probe)
	watchErr := watch.completionError(ctx, err, waitErr, settleErr)
	if watchErr != nil || waitErr != nil || settleErr != nil || ctx.Err() != nil {
		if ctx.Err() == nil {
			reportCopiedHLSProbeCompletion(parent, err, watchErr, waitErr, settleErr)
		}
		return errCopiedHLSIndex
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
