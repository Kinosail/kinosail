//go:build !linux

package playback

import "os"

func copiedAACSourceMatches(os.FileInfo,string) bool { return false }
