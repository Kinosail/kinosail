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
	initialization, err := copiedHLSCacheFile(media, "init.mp4", 2<<20)
	if err != nil {
		return err
	}
	tracks, err := copiedHLSPrivateInitialization(initialization)
	if err != nil || tracks.audio.edit != timeline.AudioOrigin.Edit || tracks.audio.scale != 48000 {
		return errCopiedHLSIndex
	}
	if _, err := media.Lstat("segment-00000.m4s"); os.IsNotExist(err) {
		return nil // Missing lazy zero retains its already bound init and packet hash.
	}
	first, err := copiedHLSCacheFile(media, "segment-00000.m4s", 64<<20)
	if err != nil {
		return err
	}
	facts, err := parseCopiedHLSPrivateAudio(ctx, initialization, first)
	bytes, decodeErr := hex.DecodeString(timeline.AudioOrigin.FirstHash[7:])
	var expected [32]byte
	copy(expected[:], bytes)
	if err != nil || decodeErr != nil || len(bytes) != 32 || facts.FirstPacket != expected ||
		facts.OriginalMediaTime != timeline.AudioOrigin.Edit || ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	return nil
}

