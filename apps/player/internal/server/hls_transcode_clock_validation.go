package server

import (
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// Apply the same bounds to generated and persisted proofs before any arithmetic
// or side effect. These limits also bound all cross-timebase integer products.
func validTranscodeHLSClock(clock *transcodeHLSClock) bool {
	source := clock.Source
	if clock.Version != 1 || len(clock.Policy) == 0 || len(clock.Policy) > 16384 || strings.ContainsAny(clock.Policy, "\r\n") || clock.Encoder == ([32]byte{}) || len(clock.Renditions) == 0 || len(clock.Renditions) > 16 {
		return false
	}
	if !validCopiedHLSAudioGrid(&copiedHLSAudioGrid{Rate: source.Rate}) || source.Base < 1000 || source.Base > math.MaxInt32 || source.Quantum <= 0 || source.Quantum > 65536 || source.Samples <= 2*source.Quantum || source.Samples > math.MaxUint32 {
		return false
	}
	uncertainty := 2*((source.Rate+source.Base-1)/source.Base) + 2
	if source.Quantum <= 2*uncertainty {
		return false
	}
	var first *transcodeRenditionClock
	for label, rendition := range clock.Renditions {
		if !playback.QualityDirectory(label) || !validTranscodeRendition(rendition, source) {
			return false
		}
		if first == nil {
			first = &rendition
			continue
		}
		if len(first.Cuts) != len(rendition.Cuts) || (first.Cuts[0].AudioPTS%1024+1024)%1024 != (rendition.Cuts[0].AudioPTS%1024+1024)%1024 {
			return false
		}
		for index, cut := range rendition.Cuts {
			if cut.VideoPTS*first.VideoBase != first.Cuts[index].VideoPTS*rendition.VideoBase {
				return false
			}
		}
	}
	return true
}

func validTranscodeRendition(clock transcodeRenditionClock, source transcodeSourceClock) bool {
	if clock.Initialization == ([32]byte{}) || clock.Manifest == ([32]byte{}) || clock.Rate != source.Rate || clock.VideoBase <= 0 || clock.VideoBase > 1_000_000 || len(clock.Cuts) == 0 || len(clock.Cuts) > 16384 {
		return false
	}
	for number, cut := range clock.Cuts {
		if cut.VideoPTS < 0 || cut.VideoPTS >= source.Samples*clock.VideoBase/source.Rate || cut.VideoDTS < cut.VideoPTS-2*clock.VideoBase || cut.VideoDTS > cut.VideoPTS || cut.AudioPTS < -1024 || cut.AudioPTS > source.Samples+1024 {
			return false
		}
		if number == 0 && cut.VideoPTS != 0 || cut.VideoPTS*source.Rate%clock.VideoBase != 0 || cut.VideoPTS*1000%clock.VideoBase != 0 {
			return false // This encoder path requires exact audio samples and millisecond video seeks.
		}
		for stream := range 2 {
			if cut.Packets[stream] <= 0 || cut.Packets[stream] > 16384 || cut.Clocks[stream] == ([32]byte{}) {
				return false
			}
		}
		if number > 0 {
			previous := clock.Cuts[number-1]
			if cut.VideoPTS <= previous.VideoPTS || cut.VideoPTS-previous.VideoPTS > 5*clock.VideoBase || cut.AudioPTS != previous.AudioPTS+int64(previous.Packets[1])*1024 {
				return false
			}
		}
		end := float64(source.Samples) / float64(source.Rate)
		if number+2 < len(clock.Cuts) {
			end = float64(clock.Cuts[number+2].VideoPTS)/float64(clock.VideoBase) + 0.5
		}
		if end-float64(cut.VideoPTS)/float64(clock.VideoBase) > 12 {
			return false
		}
	}
	return true
}

func transcodeManifestMatchesClocks(manifest []byte, clock *transcodeRenditionClock) bool {
	offset, duration, number := 0.0, 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil || !(parsed > 0 && parsed <= 12) {
				return false
			}
			duration = parsed
		} else if line != "" && !strings.HasPrefix(line, "#") {
			if number >= len(clock.Cuts) || duration <= 0 || math.Abs(offset-float64(clock.Cuts[number].VideoPTS)/float64(clock.VideoBase)) > 0.000001 {
				return false
			}
			number++
			offset += duration
			duration = 0
		}
	}
	return number == len(clock.Cuts)
}
