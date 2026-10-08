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
		{name: "byte limit rejection", mode: "bytes", wantError: true},
		{name: "line limit rejection", mode: "lines", wantError: true},
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
	markers, visits := 0, 0
	maximumBytes, maximumLines := int64(1024), 1
	if mode == "bytes" {
		maximumBytes, maximumLines = 64, 2
	}
	err = copiedHLSLines(ctx, executable, copiedHLSProbeArguments(mode), maximumBytes, maximumLines, func(line string) error {
		visits++
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "probe-started" || markers != 0 {
			t.Logf("nonkey owned probe unexpected marker length=%d fields=%d", len(line), len(fields))
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
	if markers != 1 || visits != 1 || len(processes) != 2 || (err != nil) != wantError {
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
					t.Logf("nonkey owned probe healthy unexpected-line length=%d framed-prefix=%t pass-prefix=%t run-prefix=%t", len(line), strings.HasPrefix(line, string([]byte{0x16})), strings.HasPrefix(line, "PASS"), strings.HasPrefix(line, "=== "))
					return errCopiedHLSIndex
				}
				lines++
				return nil
			})
			t.Logf("nonkey owned probe healthy mode=%s accepted-lines=%d error=%t", mode, lines, err != nil)
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
		t.Fatal("cancelled configured probe produced accepted output or succeeded")
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

func TestCopiedHLSLinesMissingExecutable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	missing := executable + "-missing-copied-probe"
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Fatal("missing executable fixture is not absent")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	lines := 0
	err = copiedHLSLines(ctx, missing, nil, 1024, 1, func(string) error {
		lines++
		return nil
	})
	if err == nil || lines != 0 {
		t.Fatal("missing configured executable was accepted")
	}
}

func TestCopiedHLSLinesInsufficientBudget(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	lines := 0
	err = copiedHLSLines(ctx, executable, copiedHLSProbeArguments("healthy"), 1024, 1, func(string) error {
		lines++
		return nil
	})
	if err == nil || lines != 0 || ctx.Err() != nil {
		t.Fatal("insufficient probe budget was accepted or consumed before refusal")
	}
}

func TestCopiedHLSLinesImmediateExit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for range 16 {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		lines := 0
		err = copiedHLSLines(ctx, executable, copiedHLSProbeArguments("healthy"), 1024, 1, func(line string) error {
			if line != "probe-ready" {
				return errCopiedHLSIndex
			}
			lines++
			return nil
		})
		cancel()
		if err != nil || lines != 1 {
			t.Fatal("immediately exiting configured probe was lost during registration")
		}
	}
}
