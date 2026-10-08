//go:build linux || darwin

package server

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

type copiedHLSUnixProbe struct {
	command *exec.Cmd
	exit    *copiedHLSExitObservation
}

// Keep the group leader unreaped until all group mutations and scanning finish.
func startCopiedHLSProbe(ctx context.Context, executable string, arguments []string) (copiedHLSProbe, io.ReadCloser, error) {
	exit, err := newCopiedHLSExitObservation()
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.Command(executable, arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, err := command.StdoutPipe()
	if err != nil {
		exit.close()
		return nil, nil, errCopiedHLSIndex
	}
	if ctx.Err() != nil || command.Start() != nil {
		_ = output.Close()
		exit.close()
		return nil, nil, errCopiedHLSIndex
	}
	probe := &copiedHLSUnixProbe{command: command, exit: exit}
	if exit.bind(command.Process.Pid) != nil {
		_ = probe.terminate()
		_ = output.Close()
		_ = probe.wait()
		_ = settleCopiedHLSProbe(ctx, probe)
		probe.close()
		return nil, nil, errCopiedHLSIndex
	}
	return probe, output, nil
}

func (probe *copiedHLSUnixProbe) exited() (bool, error) {
	return probe.exit.exited()
}

func (probe *copiedHLSUnixProbe) terminate() error {
	err := unix.Kill(-probe.command.Process.Pid, unix.SIGKILL)
	if errors.Is(err, unix.ESRCH) {
		return nil
	}
	return err
}

func (probe *copiedHLSUnixProbe) wait() error {
	return probe.command.Wait()
}

func (probe *copiedHLSUnixProbe) settled() (bool, error) {
	err := unix.Kill(-probe.command.Process.Pid, 0)
	if errors.Is(err, unix.ESRCH) {
		return true, nil
	}
	return false, err
}

func (probe *copiedHLSUnixProbe) close() {
	probe.exit.close()
}
