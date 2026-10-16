//go:build linux

package workload

import (
	"os/exec"
	"runtime"
	"syscall"
)

const backgroundNice = 10

func startBackground(command *exec.Cmd) error {
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		_ = syscall.Setpriority(syscall.PRIO_PROCESS, syscall.Gettid(), backgroundNice)
		started <- command.Start()
	}()
	return <-started
}
