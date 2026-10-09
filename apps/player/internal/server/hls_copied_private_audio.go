package server

import (
	"context"
	"crypto/sha256"
)

type copiedHLSPrivateAudioFacts struct {
	FirstRawClock     copiedHLSPrivateFirstClock
	FirstPacket       [32]byte
	Initialization    [32]byte
	First             [32]byte
	OriginalMediaTime int64
	TrackID           uint32
	Timescale         uint32
}

// These facts describe bounded byte snapshots, not a source, generation or
// readiness certificate. Rooted acquisition and all presentation proofs remain
// separate requirements before any production caller can publish these assets.
func parseCopiedHLSPrivateAudio(ctx context.Context, initialization, fragment []byte) (*copiedHLSPrivateAudioFacts, error) {
	if ctx.Err() != nil || len(initialization) == 0 || len(initialization) > 2<<20 ||
		len(fragment) == 0 || len(fragment) > 64<<20 {
		return nil, errCopiedHLSIndex
	}
	tracks, err := copiedHLSPrivateInitialization(initialization)
	if err != nil {
		return nil, err
	}
	packet, err := copiedHLSPrivateFirstAudio(ctx, fragment, tracks)
	if err != nil {
		return nil, err
	}
	clock, err := copiedHLSPrivateAudioClock(ctx, fragment, tracks.audio.id)
	if err != nil {
		return nil, err
	}
	facts := &copiedHLSPrivateAudioFacts{
		FirstRawClock:     clock,
		FirstPacket:       sha256.Sum256(packet),
		Initialization:    sha256.Sum256(initialization),
		First:             sha256.Sum256(fragment),
		OriginalMediaTime: tracks.audio.edit,
		TrackID:           tracks.audio.id,
		Timescale:         tracks.audio.scale,
	}
	if ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	return facts, nil
}
