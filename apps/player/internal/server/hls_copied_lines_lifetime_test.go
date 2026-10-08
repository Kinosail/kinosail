package server

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Populated media does not exercise probe descendants retaining or closing
// stdout. These real configured-process controls protect that coverage gap.
func TestCopiedHLSLinesOwnedLifetime(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		reject    bool
		cancel    bool
		wantError bool
	}{
		{name: "held stdout", mode: "held", wantError: true},
		{name: "early stdout closure", mode: "closed", wantError: true},
		{name: "parent exits before descendant", mode: "orphan"},
		{name: "scanner rejection", mode: "held", reject: true, wantError: true},
		{name: "explicit cancellation", mode: "held", cancel: true, wantError: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			verifyCopiedHLSProbeLifetime(t, test.mode, test.reject, test.cancel, test.wantError)
		})
	}
}

func verifyCopiedHLSProbeLifetime(t *testing.T, mode string, reject, cancelNow, wantError bool) {
	t.Helper()
	started := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var processes []copiedHLSProbeObservation
	markers := 0
	err = copiedHLSLines(ctx, executable, copiedHLSProbeArguments(mode), 1024, 1, func(line string) error {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "probe-started" || markers != 0 {
			return errCopiedHLSIndex
		}
		for _, field := range fields[1:] {
			pid, parseErr := strconv.Atoi(field)
			if parseErr != nil || pid <= 1 || pid >= 1<<30 {
				return errCopiedHLSIndex
			}
			process, observeErr := observeCopiedHLSProbe(pid)
			if observeErr != nil {
				return errCopiedHLSIndex
			}
			processes = append(processes, process)
		}
		markers++
		if cancelNow {
			cancel()
		}
		if reject {
			return errCopiedHLSIndex
		}
		return nil
	})
	t.Cleanup(func() { settleCopiedHLSProbeFixture(t, started, processes) })
	settled := copiedHLSProbeSettled(processes)
	for !settled && time.Since(started) < 2*time.Second {
		time.Sleep(time.Millisecond)
		settled = copiedHLSProbeSettled(processes)
	}
	elapsed := time.Since(started)
	t.Logf("nonkey owned probe mode=%s elapsed=%s budget=2s markers=%d parent-and-child-settled=%t error=%t", mode, elapsed, markers, settled, err != nil)
	if markers != 1 || len(processes) != 2 || (err != nil) != wantError {
		t.Fatal("configured probe did not execute its expected failure or healthy outcome")
	}
	if elapsed > 2*time.Second || !settled {
		t.Fatal("configured probe exceeded its shared budget or left an owned process")
	}
}

func TestCopiedHLSLinesHealthy(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"healthy", "healthy-closed"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			lines := 0
			err := copiedHLSLines(ctx, executable, copiedHLSProbeArguments(mode), 1024, 1, func(line string) error {
				if line != "probe-ready" {
					return errCopiedHLSIndex
				}
				lines++
				return nil
			})
			if err != nil || lines != 1 {
				t.Fatal("healthy configured probe did not complete successfully")
			}
		})
	}
}

func TestCopiedHLSLinesCancelledBeforeStart(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	lines := 0
	err = copiedHLSLines(ctx, executable, copiedHLSProbeArguments("healthy"), 1024, 1, func(string) error {
		lines++
		return nil
	})
	if err == nil || lines != 0 {
		t.Fatal("cancelled configured probe started or was accepted")
	}
}

func copiedHLSProbeSettled(processes []copiedHLSProbeObservation) bool {
	if len(processes) != 2 {
		return false
	}
	for _, process := range processes {
		if !process.settled() {
			return false
		}
	}
	return true
}

func settleCopiedHLSProbeFixture(t *testing.T, started time.Time, processes []copiedHLSProbeObservation) {
	t.Helper()
	// This is failed-fixture cleanup, never an extension of the two-second
	// acceptance budget. Observation does not signal or kill a numeric PID.
	for !copiedHLSProbeSettled(processes) && time.Since(started) < 4*time.Second {
		time.Sleep(time.Millisecond)
	}
	settled := copiedHLSProbeSettled(processes)
	for _, process := range processes {
		process.close()
	}
	t.Logf("nonkey owned probe fixture parent-and-child-settled=%t", settled)
	if len(processes) > 0 && !settled {
		t.Error("fixed three-second probe fixture did not settle within cleanup")
	}
}
