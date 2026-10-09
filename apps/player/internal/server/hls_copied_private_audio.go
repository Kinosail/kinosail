package server

import "context"

type copiedHLSPrivateAudioFacts struct {
	FirstPacket       [32]byte
	Initialization    [32]byte
	First             [32]byte
	OriginalMediaTime int64
	TrackID           uint32
	Timescale         uint32
}

// No caller, certificate or cache admission exists for private audio yet.
func parseCopiedHLSPrivateAudio(context.Context, []byte, []byte) (*copiedHLSPrivateAudioFacts, error) {
	return nil, errCopiedHLSIndex
}
