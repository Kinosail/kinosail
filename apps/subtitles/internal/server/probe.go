package server

import (
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

type (
	audioTrack     = mediaprobe.AudioTrack
	probeResult    = mediaprobe.Result
	chapter        = mediaprobe.Chapter
	playbackMarker = mediaprobe.Marker
	replayGain     = mediaprobe.ReplayGain
)

type mediaProbe struct {
	core       *mediaprobe.Probe
	executable string
	ffmpeg     string
	cacheDir   string
	markers    *markerAnalyzer
	chapters   *chapterProvider
}

func newMediaProbe(executable string) *mediaProbe {
	return &mediaProbe{core: mediaprobe.New(executable), executable: executable}
}
