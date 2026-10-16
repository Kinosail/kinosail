//go:build linux

package downloads

import (
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(destination, source *os.File) error {
	return unix.IoctlFileClone(int(destination.Fd()), int(source.Fd()))
}
