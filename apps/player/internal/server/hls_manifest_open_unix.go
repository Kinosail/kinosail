//go:build linux || darwin

package server

import (
	"math"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// os.Root.OpenFile follows confined symlinks even with caller O_NOFOLLOW.
// Use the captured directory descriptor to reject the final symlink atomically.
func openHLSManifestSnapshot(root *os.Root) (*os.File, os.FileInfo, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, nil, hlsManifestError("open")
	}
	defer directory.Close()
	parent := directory.Fd()
	if parent > math.MaxInt32 {
		return nil, nil, hlsManifestError("open")
	}
	fd, err := unix.Openat(int(parent), "index.m3u8", unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	runtime.KeepAlive(directory)
	if err != nil || fd < 0 {
		return nil, nil, hlsManifestError("open")
	}
	file := os.NewFile(uintptr(fd), "index.m3u8")
	opened, err := file.Stat()
	if err != nil || !boundedHLSManifest(opened) {
		_ = file.Close()
		return nil, nil, hlsManifestError("descriptor")
	}
	return file, opened, nil
}
