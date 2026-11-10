package playback

import (
	"runtime"
	"slices"
)

var HLSFileEvents = !slices.Contains([]string{"darwin", "dragonfly", "freebsd", "netbsd", "openbsd"}, runtime.GOOS)
