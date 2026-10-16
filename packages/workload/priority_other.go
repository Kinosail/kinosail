//go:build !linux

package workload

import "os/exec"

func startBackground(command *exec.Cmd) error { return command.Start() }
