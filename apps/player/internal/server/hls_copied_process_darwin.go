package server

import (
	"errors"

	"golang.org/x/sys/unix"
)

type copiedHLSExitObservation struct {
	queue int
	pid   int
	done  bool
}

func newCopiedHLSExitObservation() (*copiedHLSExitObservation, error) {
	queue, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(queue)
	return &copiedHLSExitObservation{queue: queue}, nil
}

func (exit *copiedHLSExitObservation) bind(pid int) error {
	exit.pid = pid
	change := unix.Kevent_t{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT}
	_, err := unix.Kevent(exit.queue, []unix.Kevent_t{change}, nil, nil)
	// This is our still-unreaped child, not a subsequently looked-up PID.
	if errors.Is(err, unix.ESRCH) {
		exit.done = true
		return nil
	}
	return err
}

func (exit *copiedHLSExitObservation) exited() (bool, error) {
	if exit.done {
		return true, nil
	}
	events := [1]unix.Kevent_t{}
	timeout := unix.Timespec{}
	count, err := unix.Kevent(exit.queue, nil, events[:], &timeout)
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	if err != nil || count == 0 {
		return false, err
	}
	event := events[0]
	if event.Ident != uint64(exit.pid) || event.Filter != unix.EVFILT_PROC || event.Flags&unix.EV_ERROR != 0 || event.Fflags&unix.NOTE_EXIT == 0 {
		return false, errCopiedHLSIndex
	}
	exit.done = true
	return true, nil
}

func (exit *copiedHLSExitObservation) close() {
	_ = unix.Close(exit.queue)
}
