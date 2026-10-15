package server

import (
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

type (
	probeResult = mediaprobe.Result
)

type mediaProbe struct {
	core       *mediaprobe.Probe
	executable string
	ffmpeg     string
	cacheDir   string
}

func newMediaProbe(executable string) *mediaProbe {
	return &mediaProbe{core: mediaprobe.New(executable), executable: executable}
}
