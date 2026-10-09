package server

import (
	"context"
	"log/slog"

	"github.com/MikeO7/kinosail/packages/library"
)

// The caller must already have joined its owned private-stage worker. This
// diagnostic operation binds observed private snapshots to a measured source
// clock; it cannot certify video, worker ownership, cache or public readiness.
func (manager *hlsManager) measureCopiedHLSPrivateSourceAudio(parent context.Context, item library.Item, recipe hlsRecipe, policy, directory string) (proof *copiedHLSAudioProof, facts *copiedHLSPrivateAudioFacts, result error) {
	failure := "metadata-admission"
	defer func() {
		if result != nil && parent.Err() == nil {
			slog.WarnContext(parent, "HLS private audio acquisition rejected",
				"request_id", requestActivityID(parent), "playback_session", requestPlaybackSession(parent), "failure_class", failure)
		}
	}()
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	defer release()
	micros, err := copiedHLSPrivateRequestedMicros(recipe.offset)
	if err != nil {
		return nil, nil, err
	}
	failure = "source-policy"
	before, err := manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, nil)
	if err != nil {
		return nil, nil, err
	}
	failure = "private-asset-acquisition"
	assets, err := manager.openCopiedHLSPrivateAssets(ctx, directory)
	if err != nil {
		return nil, nil, err
	}
	defer assets.close()
	failure = "private-asset-snapshot"
	facts, err = assets.audio(ctx)
	if err != nil {
		return nil, nil, err
	}
	failure = "source-clock"
	proof, err = manager.measureCopiedHLSSourceAudio(ctx, item, recipe, policy, facts.FirstPacket, micros, facts.OriginalMediaTime)
	if err != nil {
		return nil, nil, err
	}
	failure = "private-asset-source-binding"
	if !assets.current(ctx, facts) {
		return nil, nil, errCopiedHLSIndex
	}
	if _, err := manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, before); err != nil {
		return nil, nil, err
	}
	return proof, facts, nil
}

func copiedHLSPrivateRequestedMicros(offset float64) (int64, error) {
	if !validCopiedHLSEnd(offset) {
		return 0, errCopiedHLSIndex
	}
	micros := int64(offset * 1_000_000)
	if float64(micros)/1_000_000 != offset {
		return 0, errCopiedHLSIndex
	}
	return micros, nil
}
