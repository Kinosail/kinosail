package server

import (
	"errors"

	"golang.org/x/sys/unix"
)

type copiedHLSExitObservation struct {
	pid int
}

func newCopiedHLSExitObservation() (*copiedHLSExitObservation, error) {
	return &copiedHLSExitObservation{}, nil
}

func (exit *copiedHLSExitObservation) bind(pid int) error {
	exit.pid = pid
	return nil
}

func (exit *copiedHLSExitObservation) exited() (bool, error) {
	var info unix.Siginfo
	err := unix.Waitid(unix.P_PID, exit.pid, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil)
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return info.Signo == int32(unix.SIGCHLD) && info.Code != 0, err
}

func (exit *copiedHLSExitObservation) close() {}
