//go:build linux

package server

import (
	"fmt"
	"os"
	"syscall"
)

func copiedAACSourceToken(info os.FileInfo) (string, error) {
	if info == nil || !info.Mode().IsRegular() {
		return "", errCopiedHLSIndex
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ino == 0 {
		return "", errCopiedHLSIndex
	}
	return fmt.Sprintf("%x.%x", stat.Dev, stat.Ino), nil
}
