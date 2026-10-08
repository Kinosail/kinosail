package server

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

type copiedHLSSourceAudioSide struct {
	Kind          string `json:"side_data_type"`
	Skip          *int64 `json:"skip_samples"`
	Padding       *int64 `json:"discard_padding"`
	SkipReason    *int64 `json:"skip_reason"`
	DiscardReason *int64 `json:"discard_reason"`
}

type copiedHLSSourceAudioRow struct {
	Kind     string `json:"type"`
	PTS      *int64 `json:"pts"`
	DTS      *int64 `json:"dts"`
	Duration *int64 `json:"duration"`
	Samples  *int64 `json:"nb_samples"`
	Hash     string `json:"data_hash"`

	Side []copiedHLSSourceAudioSide `json:"side_data_list"`

	native int64
}

type copiedHLSSourceAudioStream struct {
	Codec    string `json:"codec_name"`
	Profile  string `json:"profile"`
	Rate     string `json:"sample_rate"`
	Channels int64  `json:"channels"`
	Base     string `json:"time_base"`
}

type copiedHLSSourceAudioFacts struct {
	Streams []copiedHLSSourceAudioStream `json:"streams"`

	Rows []copiedHLSSourceAudioRow `json:"packets_and_frames"`
}

type copiedHLSSourceAudioClock struct {
	denominator, leading, phase int64

	frames, packets []copiedHLSSourceAudioRow
}

// Input parsing does not grant generated-cache or subprocess admission.
func deriveCopiedHLSSourceAudio(ctx context.Context, native, normalized []byte, first [32]byte, requestedMicros, originalMediaTime int64) (*copiedHLSAudioProof, error) {
	if !validCopiedHLSSourceAudioRequest(ctx, first, requestedMicros, originalMediaTime) {
		return nil, errCopiedHLSIndex
	}
	clock, err := readCopiedHLSSourceAudio(ctx, native)
	if err != nil {
		return nil, err
	}
	rows, err := readCopiedHLSNormalizedAudio(ctx, normalized, clock)
	if err != nil {
		return nil, err
	}
	requested := requestedMicros * 48000 / 1_000_000
	proof, err := associateCopiedHLSSourceAudio(ctx, clock, rows, first, requested, originalMediaTime)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	_, _ = hash.Write(native)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(normalized)
	copy(proof.SourceClock[:], hash.Sum(nil))
	mapping := &copiedHLSPresentation{RequestedMicros: requestedMicros, Proof: &copiedHLSPresentationProof{Audio: proof}}
	if ctx.Err() != nil || !validCopiedHLSAudioProof(mapping) {
		return nil, errCopiedHLSIndex
	}
	return proof, nil
}

func validCopiedHLSSourceAudioRequest(ctx context.Context, first [32]byte, micros, original int64) bool {
	return ctx.Err() == nil && first != [32]byte{} && micros > 0 && micros <= 20_000_000 &&
		micros*48000%1_000_000 == 0 && original >= 32 && original <= 16*48000
}

func readCopiedHLSSourceAudio(ctx context.Context, data []byte) (*copiedHLSSourceAudioClock, error) {
	var facts copiedHLSSourceAudioFacts
	// Probe disposition/tags can accompany selected fields. Uniqueness and
	// bounds apply to the entire document, including these ignored fields.
	if httpguard.DecodeJSON(bytes.NewReader(data), 2<<20, &facts, false) != nil ||
		ctx.Err() != nil || len(facts.Streams) != 1 || len(facts.Rows) > 2121 {
		return nil, errCopiedHLSIndex
	}
	clock := &copiedHLSSourceAudioClock{}
	if !setCopiedHLSSourceAudioFormat(clock, facts.Streams[0]) || !collectCopiedHLSSourceAudioRows(clock, facts.Rows) {
		return nil, errCopiedHLSIndex
	}
	if !validateCopiedHLSSourceAudioFrames(ctx, clock) || !validateCopiedHLSSourceAudioPackets(ctx, clock) {
		return nil, errCopiedHLSIndex
	}
	return clock, nil
}

func collectCopiedHLSSourceAudioRows(clock *copiedHLSSourceAudioClock, rows []copiedHLSSourceAudioRow) bool {
	for _, row := range rows {
		switch row.Kind {
		case "frame":
			clock.frames = append(clock.frames, row)
		case "packet":
			clock.packets = append(clock.packets, row)
		default:
			return false
		}
	}
	return true
}

func setCopiedHLSSourceAudioFormat(clock *copiedHLSSourceAudioClock, stream copiedHLSSourceAudioStream) bool {
	if stream.Codec != "aac" || stream.Profile != "LC" || stream.Rate != "48000" || stream.Channels != 2 {
		return false
	}
	switch stream.Base {
	case "1/1000":
		clock.denominator = 1000
	case "1/48000":
		clock.denominator = 48000
	default:
		return false
	}
	return true
}

func validateCopiedHLSSourceAudioFrames(ctx context.Context, clock *copiedHLSSourceAudioClock) bool {
	if len(clock.frames) < 1024 || len(clock.frames) > 1060 {
		return false
	}
	if !setCopiedHLSSourceAudioBootstrap(clock) {
		return false
	}
	var ordinal int64
	for number := range clock.frames {
		row := &clock.frames[number]
		count := int64(1024)
		if number == 0 {
			count = clock.leading
		}
		if ctx.Err() != nil || !validCopiedHLSSourceAudioFrame(*row, clock, number, ordinal, count) {
			return false
		}
		row.native = ordinal
		ordinal += count
	}
	return true
}

func setCopiedHLSSourceAudioBootstrap(clock *copiedHLSSourceAudioClock) bool {
	if clock.frames[0].Samples == nil {
		return false
	}
	clock.leading = *clock.frames[0].Samples
	switch clock.leading {
	case 1024:
		return true
	case 16:
		clock.phase = -8
		return clock.denominator == 48000
	default:
		return false
	}
}

func validCopiedHLSSourceAudioFrame(row copiedHLSSourceAudioRow, clock *copiedHLSSourceAudioClock, number int, ordinal, count int64) bool {
	if row.PTS == nil || row.Samples == nil || *row.Samples != count || len(row.Side) != 0 {
		return false
	}
	if !copiedHLSSourceAudioFrameShape(row, clock.denominator) {
		return false
	}
	errorSamples := *row.PTS*48000 - (ordinal+clock.samplePhase(number))*clock.denominator
	return errorSamples >= -24000 && errorSamples <= 24000
}

func copiedHLSSourceAudioFrameShape(row copiedHLSSourceAudioRow, denominator int64) bool {
	return *row.PTS >= 0 && *row.PTS <= 22*denominator &&
		row.DTS == nil && row.Duration == nil && row.Hash == ""
}

func (clock *copiedHLSSourceAudioClock) samplePhase(number int) int64 {
	if number >= 2 {
		return clock.phase
	}
	return 0
}
