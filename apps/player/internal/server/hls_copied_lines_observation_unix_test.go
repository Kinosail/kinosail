//go:build !windows

package server

import (
	"errors"
	"os"
	"syscall"
)

type copiedHLSProbeObservation struct {
	process *os.Process
}

func observeCopiedHLSProbe(pid int) (copiedHLSProbeObservation, error) {
	process, err := os.FindProcess(pid)
	return copiedHLSProbeObservation{process: process}, err
}

func (observation copiedHLSProbeObservation) settled() bool {
	return errors.Is(observation.process.Signal(syscall.Signal(0)), os.ErrProcessDone)
}

func (observation copiedHLSProbeObservation) close() {
	_ = observation.process.Release()
}
