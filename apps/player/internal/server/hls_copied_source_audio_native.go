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
	if err != nil || !copiedHLSSourceAudioInitialPacket(clock, first) {
		return nil, errCopiedHLSIndex
	}
	return native, nil
}

func copiedHLSSourceAudioInitialPacket(clock *copiedHLSSourceAudioClock, first [32]byte) bool {
	frame := copiedHLSSourceAudioPacketIndex(clock, first)
	if clock.leading == 1024 {
		frame--
	}
	return frame >= 2
}
