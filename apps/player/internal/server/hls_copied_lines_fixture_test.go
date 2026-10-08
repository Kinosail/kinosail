package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func copiedHLSProbeArguments(mode string) []string {
	return []string{"-test.run=^TestCopiedHLSProbeFixture$", "--", "kinosail-probe-fixture:" + mode}
}

func TestCopiedHLSProbeFixture(t *testing.T) {
	mode := ""
	for _, argument := range os.Args {
		if value, found := strings.CutPrefix(argument, "kinosail-probe-fixture:"); found {
			mode = value
		}
	}
	if mode == "" {
		return
	}
	if mode == "linger" {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
	if mode == "healthy" || mode == "healthy-closed" {
		_, _ = fmt.Fprintln(os.Stdout, "probe-ready")
		if mode == "healthy-closed" {
			_ = os.Stdout.Close()
			time.Sleep(30 * time.Millisecond)
		}
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	child := exec.Command(executable, copiedHLSProbeArguments("linger")...)
	if mode != "closed" {
		child.Stdout = os.Stdout
	}
	if child.Start() != nil {
		os.Exit(3)
	}
	_, _ = fmt.Fprintf(os.Stdout, "probe-started %d %d\n", os.Getpid(), child.Process.Pid)
	if mode == "closed" {
		_ = os.Stdout.Close()
	}
	if mode == "orphan" {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}
	if child.Wait() != nil {
		os.Exit(4)
	}
	os.Exit(0)
}
