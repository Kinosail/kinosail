package server

import "context"

// Complete source rows and the actual first private packet must be associated
// before any measured audio tuple can be attached to generated assets.
func deriveCopiedHLSSourceAudio(ctx context.Context, native, normalized []byte, first [32]byte, requestedMicros, originalMediaTime int64) (*copiedHLSAudioProof, error) {
	return nil, errCopiedHLSIndex
}
