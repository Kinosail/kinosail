package server

import "context"

// Reject incomplete rows or absent private-packet identity before spending a
// second source read. Final derivation still applies every strict clock check.
func copiedHLSSourceAudioNative(ctx context.Context, executable, source string, first [32]byte) ([]byte, error) {
	native, err := copiedHLSSourceAudioOutput(ctx, executable, copiedHLSSourceAudioProbeArguments(source), 24_000)
	if err != nil {
		return nil, err
	}
	clock, err := readCopiedHLSSourceAudio(ctx, native)
	if err != nil || copiedHLSSourceAudioPacketIndex(clock, first) < 1 {
		return nil, errCopiedHLSIndex
	}
	return native, nil
}
