package server

import (
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

func validTranscodeHLSClock(clock *transcodeHLSClock) bool {
	source := clock.Source
	if !validTranscodeClockEnvelope(clock) {
		return false
	}
	if !validTranscodeSourceClock(source) {
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
		if !sameTranscodeRenditionCuts(*first, rendition) {
			return false
		}
	}
	return true
}

func validTranscodeClockEnvelope(clock *transcodeHLSClock) bool {
	return clock.Version == 1 && len(clock.Policy) > 0 && len(clock.Policy) <= 16384 && !strings.ContainsAny(clock.Policy, "\r\n") && clock.Encoder != ([32]byte{}) && len(clock.Renditions) > 0 && len(clock.Renditions) <= 16
}

func validTranscodeSourceClock(source transcodeSourceClock) bool {
	if !validCopiedHLSAudioGrid(&copiedHLSAudioGrid{Rate: source.Rate}) || source.Base < 1000 || source.Base > math.MaxInt32 || source.Quantum <= 0 || source.Quantum > 65536 || source.Samples <= 2*source.Quantum || source.Samples > math.MaxUint32 {
		return false
	}
	uncertainty := 2*((source.Rate+source.Base-1)/source.Base) + 2
	return source.Quantum > 2*uncertainty
}

func sameTranscodeRenditionCuts(first, other transcodeRenditionClock) bool {
	if len(first.Cuts) != len(other.Cuts) || (first.Cuts[0].AudioPTS%1024+1024)%1024 != (other.Cuts[0].AudioPTS%1024+1024)%1024 {
		return false
	}
	for index, cut := range other.Cuts {
		if cut.VideoPTS*first.VideoBase != first.Cuts[index].VideoPTS*other.VideoBase {
			return false
		}
	}
	return true
}

func validTranscodeRendition(clock transcodeRenditionClock, source transcodeSourceClock) bool {
	if !validTranscodeRenditionHeader(clock, source.Rate) {
		return false
	}
	for number, cut := range clock.Cuts {
		if !validTranscodeFragment(cut, number, clock.VideoBase, source) {
			return false
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
		warm := transcodeWarmSample(cut.VideoPTS*source.Rate/clock.VideoBase, source.Rate, clock.Cuts[0].AudioPTS)
		if end-float64(warm)/float64(source.Rate) > 12 {
			return false
		}
	}
	return true
}

func validTranscodeRenditionHeader(clock transcodeRenditionClock, rate int64) bool {
	return clock.Initialization != ([32]byte{}) && clock.Manifest != ([32]byte{}) && clock.Rate == rate && clock.VideoBase > 0 && clock.VideoBase <= 1_000_000 && len(clock.Cuts) > 0 && len(clock.Cuts) <= 16384
}

func validTranscodeFragment(cut transcodeFragmentClock, number int, base int64, source transcodeSourceClock) bool {
	if !cut.withinTranscodeSource(base, source) {
		return false
	}
	if number == 0 && cut.VideoPTS != 0 || cut.VideoPTS*source.Rate%base != 0 || cut.VideoPTS*1000%base != 0 {
		return false
	}
	for stream := range 2 {
		if cut.Packets[stream] <= 0 || cut.Packets[stream] > 16384 || cut.Clocks[stream] == ([32]byte{}) {
			return false
		}
	}
	return true
}

func (cut *transcodeFragmentClock) withinTranscodeSource(base int64, source transcodeSourceClock) bool {
	return cut.VideoPTS >= 0 && cut.VideoPTS < source.Samples*base/source.Rate && cut.VideoDTS >= cut.VideoPTS-2*base && cut.VideoDTS <= cut.VideoPTS && cut.AudioPTS >= -1024 && cut.AudioPTS <= source.Samples+1024
}

func transcodeManifestMatchesClocks(manifest []byte, clock *transcodeRenditionClock) bool {
	offset, duration, number := 0.0, 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			parsed := transcodeManifestDuration(line)
			if parsed == 0 {
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

func transcodeManifestDuration(line string) float64 {
	value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || !(parsed > 0 && parsed <= 12) {
		return 0
	}
	return parsed
}
