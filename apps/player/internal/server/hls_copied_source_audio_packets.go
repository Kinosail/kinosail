package server

import (
	"context"
	"encoding/hex"
	"strings"
)

func validateCopiedHLSSourceAudioPackets(ctx context.Context, clock *copiedHLSSourceAudioClock) bool {
	extra := 0
	if clock.leading == 1024 {
		extra = 1
	}
	if len(clock.packets) != len(clock.frames)+extra || !validCopiedHLSSourceAudioPrime(clock.packets[0], clock) {
		return false
	}
	for number := 1; number < len(clock.packets); number++ {
		row := clock.packets[number]
		frame := clock.frames[number-extra]
		if ctx.Err() != nil || !validCopiedHLSSourceAudioPacket(row, frame, number, clock) {
			return false
		}
	}
	return true
}

func validCopiedHLSSourceAudioPrime(row copiedHLSSourceAudioRow, clock *copiedHLSSourceAudioClock) bool {
	if !copiedHLSSourceAudioPacketShape(row) || len(row.Side) != 1 {
		return false
	}
	skip := int64(1024)
	if clock.leading == 16 {
		skip = 1008
	}
	expected := -(skip*clock.denominator + 24000) / 48000
	if *row.PTS != expected || !copiedHLSSourceAudioDuration(*row.Duration, 1024, clock.denominator) {
		return false
	}
	return validCopiedHLSSourceAudioSkip(row.Side[0], skip)
}

func validCopiedHLSSourceAudioSkip(side copiedHLSSourceAudioSide, skip int64) bool {
	if side.Kind != "Skip Samples" || side.Skip == nil || side.Padding == nil || side.SkipReason == nil || side.DiscardReason == nil {
		return false
	}
	return *side.Skip == skip && *side.Padding == 0 && *side.SkipReason == 0 && *side.DiscardReason == 0
}

func copiedHLSSourceAudioPacketShape(row copiedHLSSourceAudioRow) bool {
	return row.PTS != nil && row.DTS != nil && row.Duration != nil && *row.PTS == *row.DTS &&
		row.Samples == nil && copiedHLSSourceAudioHash(row.Hash) != [32]byte{}
}

func validCopiedHLSSourceAudioPacket(row, frame copiedHLSSourceAudioRow, number int, clock *copiedHLSSourceAudioClock) bool {
	if !copiedHLSSourceAudioPacketShape(row) || len(row.Side) != 0 || *row.PTS != *frame.PTS {
		return false
	}
	count := int64(1024)
	if clock.leading == 16 && number == 1 {
		count = 1016
	}
	return copiedHLSSourceAudioDuration(*row.Duration, count, clock.denominator)
}

func copiedHLSSourceAudioDuration(duration, samples, denominator int64) bool {
	if duration <= 0 || duration > 1024 {
		return false
	}
	difference := duration*48000 - samples*denominator
	return difference >= -24000 && difference <= 24000
}

func copiedHLSSourceAudioHash(value string) [32]byte {
	var result [32]byte
	encoded, ok := strings.CutPrefix(value, "SHA256:")
	if !ok || len(encoded) != 64 {
		return result
	}
	data, err := hex.DecodeString(encoded)
	if err != nil || hex.EncodeToString(data) != encoded {
		return result
	}
	copy(result[:], data)
	return result
}

func copiedHLSSourceAudioPacketIndex(clock *copiedHLSSourceAudioClock, first [32]byte) int {
	packet := -1
	for number, row := range clock.packets {
		if copiedHLSSourceAudioHash(row.Hash) == first {
			if packet >= 0 {
				return -1
			}
			packet = number
		}
	}
	return packet
}

func copiedHLSSourceAudioTarget(rows []copiedHLSNormalizedAudioRow, requested int64) int {
	target := -1
	for number, row := range rows {
		if row.pts <= requested && requested < row.pts+row.samples {
			if target >= 0 {
				return -1
			}
			target = number
		}
	}
	return target
}

func associateCopiedHLSSourceAudio(ctx context.Context, clock *copiedHLSSourceAudioClock, rows []copiedHLSNormalizedAudioRow, first [32]byte, requested, original int64) (*copiedHLSAudioProof, error) {
	packet := copiedHLSSourceAudioPacketIndex(clock, first)
	target := copiedHLSSourceAudioTarget(rows, requested)
	if ctx.Err() != nil || packet < 1 || target < 2 || target+1 >= len(clock.frames) {
		return nil, errCopiedHLSIndex
	}
	firstFrame := packet
	if clock.leading == 1024 {
		firstFrame--
	}
	if firstFrame < 2 || firstFrame > target {
		return nil, errCopiedHLSIndex
	}
	frame, selected := clock.frames[firstFrame], clock.frames[target]
	wanted := selected.native + requested - rows[target].pts
	return &copiedHLSAudioProof{
		Codec: "aac", Profile: "LC", SampleRate: 48000, Channels: 2,
		Numerator: 1, Denominator: clock.denominator, FirstPacket: first,
		FirstPTS: *frame.PTS, FirstNativeSample: frame.native, RequestedSample: requested,
		TargetPTS: rows[target].pts, TargetNativeSample: selected.native, TargetSamples: rows[target].samples,
		LeadingSamples: clock.leading, SourcePhase: clock.phase,
		MediaTime: wanted - frame.native, OriginalMediaTime: original,
	}, nil
}
