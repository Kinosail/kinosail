//go:build linux

package playback

import (
	"fmt"
	"os"
	"syscall"
)

func copiedAACSourceMatches(info os.FileInfo, token string) bool {
	value, ok := info.Sys().(*syscall.Stat_t)
	return ok && value.Ino != 0 && fmt.Sprintf("%x.%x", value.Dev, value.Ino) == token
}
