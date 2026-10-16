package workload

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func childNice(t *testing.T, run func(*exec.Cmd) ([]byte, error)) int {
	t.Helper()
	output, err := run(exec.CommandContext(t.Context(), "/bin/sh", "-c", "cut -d ' ' -f 19 /proc/self/stat"))
	if err != nil {
		t.Fatal(err)
	}
	nice, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatalf("nice = %q: %v", output, err)
	}
	return nice
}

func TestBackgroundChildrenRunAtLowerPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process niceness is Linux-specific")
	}
	parent := childNice(t, func(command *exec.Cmd) ([]byte, error) { return command.Output() })
	background := childNice(t, BackgroundOutput)
	if background < min(19, max(parent, 10)) {
		t.Fatalf("background child nice = %d, parent child nice = %d", background, parent)
	}
	for range 4 {
		if after := childNice(t, func(command *exec.Cmd) ([]byte, error) { return command.Output() }); after != parent {
			t.Fatalf("playback child inherited background priority: %d, want %d", after, parent)
		}
	}
	combined, err := BackgroundCombinedOutput(exec.CommandContext(t.Context(), "/bin/sh", "-c", "printf out; printf err >&2"))
	if err != nil || string(combined) != "outerr" {
		t.Fatalf("combined output = %q, %v", combined, err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := RunBackground(exec.CommandContext(canceled, "/bin/sh", "-c", "exit 0")); err == nil {
		t.Fatal("canceled background child started")
	}
	if err := RunContext(WithBackgroundPriority(t.Context()), exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 3")); err == nil {
		t.Fatal("failed background child reported success")
	}
	if !BackgroundPriority(WithBackgroundPriority(t.Context())) || BackgroundPriority(t.Context()) {
		t.Fatal("background priority context marker is inconsistent")
	}
}
