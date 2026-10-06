//go:build unix

package mediaprobe

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A concurrent fork can inherit a writable descriptor before close-on-exec.
// Keep it out of fixture publication, then release before command execution.
// https://github.com/golang/go/issues/62221
func lockProbeFixturePublication() func() {
	syscall.ForkLock.RLock()
	return syscall.ForkLock.RUnlock
}

// Populated media E2E uses installed executables and cannot expose the test
// fixture write/exec boundary. Preserve strict command bytes under concurrent
// publication; a newly written command must execute once without retries.
func TestProbeFixturesExecuteDuringParallelPublication(t *testing.T) {
	t.Parallel()
	for worker := range 8 {
		t.Run(fmt.Sprintf("worker-%d", worker), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			root := t.TempDir()
			for publication := range 64 {
				executable := filepath.Join(root, fmt.Sprintf("probe-%d", publication))
				expected := fmt.Sprintf("worker-%d-publication-%d", worker, publication)
				writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '"+expected+"'\n")
				//nolint:gosec // The executable is a private, generated test fixture.
				output, err := exec.CommandContext(ctx, executable).Output()
				if err != nil || string(output) != expected {
					t.Fatalf("fixture worker=%d publication=%d output=%q error=%v", worker, publication, output, err)
				}
			}
		})
	}
}
