package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

func (output *transcodeHLSOutput) publish(manager *hlsManager, ctx context.Context, item library.Item, directory, policy string) error {
	recovery := output.recovery
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errTranscodeHLSClock
	}
	defer root.Close()
	media, err := root.OpenRoot(recovery.rendition)
	if err != nil {
		return errTranscodeHLSClock
	}
	defer media.Close()
	stage, err := output.stage.OpenRoot(recovery.rendition)
	if err != nil {
		return errTranscodeHLSClock
	}
	defer stage.Close()
	name := transcodeFragmentName(recovery.number)
	measured, err := manager.probeTranscodeFragments(ctx, stage, []string{name})
	original := recovery.clock.Renditions[recovery.rendition]
	if err != nil || !matchesTranscodeFragment(measured, original, recovery.number) {
		return errTranscodeHLSClock
	}
	current := func() bool {
		if manager.validateHLSPolicy(ctx, item, recovery.original, policy) != nil || !manager.copiedHLSCanonicalGeneration(directory, recovery.rendition, root, media) {
			return false
		}
		again, err := manager.readTranscodeHLSRecovery(ctx, directory, policy, recovery.rendition, recovery.number)
		return err == nil && again != nil && again.digest == recovery.digest
	}
	if !current() {
		return errTranscodeHLSClock
	}
	file, _, err := copiedHLSOpenFile(stage, name, 64<<20)
	if err != nil {
		return errTranscodeHLSClock
	}
	defer file.Close()
	pendingName := filepath.Base(output.directory) + ".pending"
	pending, err := media.OpenFile(pendingName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errTranscodeHLSClock
	}
	owned, err := pending.Stat()
	if err != nil {
		_ = pending.Close()
		return errTranscodeHLSClock
	}
	defer func() {
		_ = pending.Close()
		copiedHLSRemoveOwned(media, pendingName, owned)
	}()
	if copyCopiedHLSPending(ctx, file, pending) != nil {
		return errTranscodeHLSClock
	}
	checked, err := manager.probeTranscodeFragments(ctx, media, []string{pendingName})
	physical, statErr := media.Lstat(pendingName)
	if err != nil || statErr != nil || !os.SameFile(owned, physical) || !matchesTranscodeFragment(checked, original, recovery.number) || !current() {
		return errTranscodeHLSClock
	}
	linkErr := media.Link(pendingName, name)
	if linkErr != nil && !errors.Is(linkErr, os.ErrExist) {
		return errTranscodeHLSClock
	}
	// A race winner remains untouched. Its clock must independently agree with
	// the original generation before this waiter can accept it.
	committed, err := manager.probeTranscodeFragments(ctx, media, []string{name})
	if err != nil || !matchesTranscodeFragment(committed, original, recovery.number) || !current() {
		if linkErr == nil {
			copiedHLSRemoveOwned(media, name, owned)
		}
		return errTranscodeHLSClock
	}
	return nil
}

func matchesTranscodeFragment(actual *transcodeRenditionClock, original transcodeRenditionClock, number int) bool {
	return actual != nil && len(actual.Cuts) == 1 && actual.Initialization == original.Initialization && actual.Rate == original.Rate && actual.VideoBase == original.VideoBase && actual.Cuts[0] == original.Cuts[number] && actual.Initialization != ([sha256.Size]byte{})
}
