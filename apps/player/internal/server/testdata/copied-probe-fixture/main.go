package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(2)
	}
	mode := os.Args[1]
	if mode == "linger" {
		time.Sleep(3 * time.Second)
		return
	}
	if mode == "healthy" || mode == "healthy-closed" {
		_, _ = fmt.Fprintln(os.Stdout, "probe-ready")
		if mode == "healthy-closed" {
			_ = os.Stdout.Close()
			time.Sleep(30 * time.Millisecond)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(3)
	}
	child := exec.Command(executable, "linger")
	if mode != "closed" {
		child.Stdout = os.Stdout
	}
	if child.Start() != nil {
		os.Exit(4)
	}
	_, _ = fmt.Fprintf(os.Stdout, "probe-started %d %d\n", os.Getpid(), child.Process.Pid)
	if mode == "closed" {
		_ = os.Stdout.Close()
	}
	if mode == "bytes" {
		_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("x", 64))
	}
	if mode == "lines" {
		_, _ = fmt.Fprintln(os.Stdout, "probe-extra")
	}
	if mode == "orphan" {
		time.Sleep(300 * time.Millisecond)
		return
	}
	if child.Wait() != nil {
		os.Exit(5)
	}
}
