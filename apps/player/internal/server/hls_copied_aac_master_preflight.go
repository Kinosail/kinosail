package server

import (
	"bytes"
	"context"
	"os"

	"github.com/MikeO7/kinosail/packages/library"
)

// This read-only preflight ends before preparation can wait for an owner or edit
// the cache. Pending clocks retain the producer's certificate-first interval.
func (manager *hlsManager) copiedAACMasterPreflight(parent context.Context, item library.Item, recipe hlsRecipe, directory, policy string) error {
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer release()
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	master, entry, timeline, err := copiedAACPresentMaster(ctx, root, policy)
	if err != nil {
		return err
	}
	if timeline == nil {
		return nil // Preserve the existing initial-owner path while master is absent.
	}
	if timeline.Clock == nil {
		return manager.copiedAACPendingMasterCurrent(ctx, item, recipe, directory, root, entry.rendition, policy)
	}
	return manager.copiedAACBoundMasterCurrent(ctx, item, recipe, directory, root, master)
}

func copiedAACPresentMaster(ctx context.Context, root *os.Root, policy string) ([]byte, copiedAACMasterEntry, *copiedHLSTimeline, error) {
	data, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(data) != policy || ctx.Err() != nil {
		return nil, copiedAACMasterEntry{}, nil, errCopiedHLSIndex
	}
	if _, err := root.Lstat("index.m3u8"); os.IsNotExist(err) {
		return nil, copiedAACMasterEntry{}, nil, nil
	}
	master, err := copiedHLSCacheFile(root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	entry, valid := parseCopiedAACMaster(master, policy)
	if err != nil || !valid {
		return nil, entry, nil, errCopiedHLSIndex
	}
	timeline, _, err := decodeCopiedHLSTimeline(root, policy)
	if err != nil || timeline.AudioOrigin == nil || timeline.Presentation != nil {
		return nil, entry, nil, errCopiedHLSIndex
	}
	return master, entry, timeline, nil
}

func (manager *hlsManager) copiedAACPendingMasterCurrent(ctx context.Context, item library.Item, recipe hlsRecipe, directory string, root *os.Root, rendition, policy string) error {
	media, err := root.OpenRoot(rendition)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer media.Close()
	if ctx.Err() != nil || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil ||
		!manager.copiedHLSCanonicalGeneration(directory, rendition, root, media) {
		return errCopiedHLSIndex
	}
	// No certificate/timeline hash agreement is required before the bound commit.
	return nil
}

func (manager *hlsManager) copiedAACBoundMasterCurrent(ctx context.Context, item library.Item, recipe hlsRecipe, directory string, root *os.Root, master []byte) error {
	held, err := manager.openCopiedAACGeneration(ctx, item, recipe, directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer held.close()
	if !manager.copiedHLSCanonicalGeneration(directory, held.certificate.Rendition, root, held.media) {
		return errCopiedHLSIndex
	}
	current, err := copiedHLSCacheFile(held.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !bytes.Equal(master, current) || !held.masterAllowed(current) || !held.current() {
		return errCopiedHLSIndex
	}
	return nil
}
