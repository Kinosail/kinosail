package server

import (
	"os"
	"sync/atomic"
)

type copiedHLSProbeObservation struct {
	done   chan struct{}
	exited *atomic.Bool
}

func observeCopiedHLSProbe(pid int) (copiedHLSProbeObservation, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return copiedHLSProbeObservation{}, err
	}
	observation := copiedHLSProbeObservation{done: make(chan struct{}), exited: new(atomic.Bool)}
	go func() {
		state, waitErr := process.Wait()
		observation.exited.Store(state != nil && waitErr == nil)
		close(observation.done)
	}()
	return observation, nil
}

func (observation copiedHLSProbeObservation) settled() bool {
	select {
	case <-observation.done:
		return observation.exited.Load()
	default:
		return false
	}
}

func (observation copiedHLSProbeObservation) close() {}
