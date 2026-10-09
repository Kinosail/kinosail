package server

import (
	"context"
	"os"

	"github.com/MikeO7/kinosail/packages/library"
)

// This unreferenced contract seam preserves the current pathname-only reader.
// It must remain uncalled and unpublished until rooted-acquisition RED proof.
type copiedHLSPrivateAssets struct{ root *os.Root }

func (manager *hlsManager) openCopiedHLSPrivateAssets(ctx context.Context, directory string) (*copiedHLSPrivateAssets, error) {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return nil, err
	}
	return &copiedHLSPrivateAssets{root: root}, nil
}

func (assets *copiedHLSPrivateAssets) audio(ctx context.Context) (*copiedHLSPrivateAudioFacts, error) {
	initialization, err := copiedHLSCacheFile(assets.root, "init.mp4", 2<<20)
	if err != nil {
		return nil, err
	}
	first, err := copiedHLSCacheFile(assets.root, "segment-00000.m4s", 64<<20)
	if err != nil {
		return nil, err
	}
	return parseCopiedHLSPrivateAudio(ctx, initialization, first)
}

func (assets *copiedHLSPrivateAssets) current(ctx context.Context, facts *copiedHLSPrivateAudioFacts) bool {
	return ctx.Err() == nil
}

func (assets *copiedHLSPrivateAssets) close() { _ = assets.root.Close() }

func (manager *hlsManager) measureCopiedHLSPrivateSourceAudio(ctx context.Context, item library.Item, recipe hlsRecipe, policy, directory string) (*copiedHLSAudioProof, *copiedHLSPrivateAudioFacts, error) {
	return nil, nil, errCopiedHLSIndex
}
