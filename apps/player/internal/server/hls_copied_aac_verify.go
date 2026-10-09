package server

import (
	"context"
	"encoding/hex"
	"os"
)

func verifyCopiedAACAssets(ctx context.Context, media *os.Root, timeline *copiedHLSTimeline) error {
	if timeline.AudioOrigin == nil {
		return nil
	}
	if !validCopiedAACOrigin(timeline) || timeline.Clock == nil {
		return errCopiedHLSIndex
	}
	initialization, err := copiedAACVerifiedInitialization(media, timeline.AudioOrigin)
	if err != nil {
		return err
	}
	if _, err := media.Lstat("segment-00000.m4s"); os.IsNotExist(err) {
		return nil // Missing lazy zero retains its already bound init and packet hash.
	}
	first, err := copiedHLSCacheFile(media, "segment-00000.m4s", 64<<20)
	if err != nil {
		return err
	}
	return verifyCopiedAACFirst(ctx, initialization, first, timeline.AudioOrigin)
}

func copiedAACVerifiedInitialization(media *os.Root, origin *copiedHLSAudioOrigin) ([]byte, error) {
	initialization, err := copiedHLSCacheFile(media, "init.mp4", 2<<20)
	if err != nil {
		return nil, err
	}
	tracks, err := copiedHLSPrivateInitialization(initialization)
	if err != nil || tracks.audio.edit != origin.Edit || tracks.audio.scale != 48000 {
		return nil, errCopiedHLSIndex
	}
	return initialization, nil
}

func verifyCopiedAACFirst(ctx context.Context, initialization, first []byte, origin *copiedHLSAudioOrigin) error {
	facts, err := parseCopiedHLSPrivateAudio(ctx, initialization, first)
	bytes, decodeErr := hex.DecodeString(origin.FirstHash[7:])
	var expected [32]byte
	copy(expected[:], bytes)
	if err != nil || decodeErr != nil || len(bytes) != 32 || ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	if !validCopiedAACPhysicalWitness(facts, expected, origin.Edit) {
		return errCopiedHLSIndex
	}
	return nil
}

func validCopiedAACPhysicalWitness(facts *copiedHLSPrivateAudioFacts, expected [32]byte, edit int64) bool {
	return facts.FirstPacket == expected && facts.OriginalMediaTime == edit && facts.FirstRawClock.Decode == 0 &&
		facts.FirstRawClock.Composition == 0 && facts.FirstRawClock.Duration == 1024
}
