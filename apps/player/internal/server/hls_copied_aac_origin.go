package server

import (
	"encoding/hex"
	"strings"
)

const maximumCopiedAACMicros int64 = 7 * 24 * 60 * 60 * 1_000_000
const maximumCopiedAACTicks int64 = 7 * 24 * 60 * 60 * 48000

// Generated proof is separate from the retained source eligibility decision.
type copiedHLSAudioOrigin struct {
	SourceTrack      int
	InitialSeekMicros int64
	FirstPTS         int64
	FirstHash        string
	Physical         int64
	Edit             int64
}

type copiedHLSAudioPacket struct {
	PTS, DTS, Duration int64
	Hash               string
}

func validCopiedAACHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "SHA256:") {
		return false
	}
	data, err := hex.DecodeString(value[7:])
	return err == nil && len(data) == 32 && strings.ToLower(value) == "sha256:"+value[7:]
}

func validCopiedAACPacket(packet copiedHLSAudioPacket) bool {
	return packet.PTS >= -48000 && packet.PTS <= maximumCopiedAACTicks && packet.DTS == packet.PTS &&
		packet.Duration == 1024 && validCopiedAACHash(packet.Hash)
}

func copiedAACRescale(micros int64) (int64, error) {
	if micros < 0 || micros > maximumCopiedAACMicros {
		return 0, errCopiedHLSIndex
	}
	return (micros*48000 + 500000) / 1_000_000, nil
}

func copiedHLSAudioWitness(source []copiedHLSAudioPacket, edited, raw copiedHLSAudioPacket, seekMicros int64) (*copiedHLSAudioOrigin, error) {
	seek, err := copiedAACRescale(seekMicros)
	if err != nil || len(source) == 0 || len(source) > 128 || !validCopiedAACPacket(edited) ||
		!validCopiedAACPacket(raw) || raw.PTS != 0 || raw.Hash != edited.Hash {
		return nil, errCopiedHLSIndex
	}
	edit := raw.PTS - edited.PTS
	if edit < 0 || edit > 48000 || raw.DTS-edited.DTS != edit {
		return nil, errCopiedHLSIndex
	}
	var first copiedHLSAudioPacket
	matches := 0
	for _, packet := range source {
		if !validCopiedAACPacket(packet) {
			return nil, errCopiedHLSIndex
		}
		if packet.Hash == raw.Hash {
			first = packet
			matches++
		}
	}
	if matches != 1 || first.PTS+edit != seek {
		return nil, errCopiedHLSIndex
	}
	return &copiedHLSAudioOrigin{InitialSeekMicros: seekMicros, FirstPTS: first.PTS, FirstHash: first.Hash, Physical: -first.PTS, Edit: edit}, nil
}

func copiedHLSAudioShift(physical, seekMicros, muxMicros int64) (int64, error) {
	seek, seekErr := copiedAACRescale(seekMicros)
	mux, muxErr := copiedAACRescale(muxMicros)
	if seekErr != nil || muxErr != nil || physical < -maximumCopiedAACTicks || physical > 48000 {
		return 0, errCopiedHLSIndex
	}
	shift := physical + seek - mux
	if shift < -48000 || shift > 48000 {
		return 0, errCopiedHLSIndex
	}
	return shift, nil
}

func copiedHLSCertificateVersion(timeline *copiedHLSTimeline) int {
	if timeline != nil && timeline.AudioOrigin != nil {
		return 2
	}
	return 1
}

func validCopiedAACOrigin(timeline *copiedHLSTimeline) bool {
	origin := timeline.AudioOrigin
	if origin == nil {
		return true
	}
	if timeline.Presentation != nil || timeline.Strategy != "h264-idr-keys-1" || !strings.HasSuffix(timeline.Policy, ":copied-aac=2") ||
		origin.SourceTrack < 0 || origin.SourceTrack > 255 || origin.InitialSeekMicros < 0 || origin.InitialSeekMicros > maximumCopiedAACMicros {
		return false
	}
	if len(timeline.Keys) == 0 {
		return false
	}
	initial, err := copiedAACKeyMicros(timeline, timeline.Keys[0].PTS)
	if err != nil || initial != origin.InitialSeekMicros {
		return false
	}
	if timeline.Clock == nil {
		return origin.FirstHash == "" && origin.FirstPTS == 0 && origin.Physical == 0 && origin.Edit == 0
	}
	seek, err := copiedAACRescale(origin.InitialSeekMicros)
	return err == nil && *timeline.Clock == 0 && validCopiedAACHash(origin.FirstHash) && origin.FirstPTS >= -48000 &&
		origin.FirstPTS <= maximumCopiedAACTicks && origin.Physical == -origin.FirstPTS &&
		origin.Edit >= 0 && origin.Edit <= 48000 && origin.FirstPTS+origin.Edit == seek
}
