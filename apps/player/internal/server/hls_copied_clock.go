package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSProbeOutput struct {
	bytes.Buffer
}

// A complete, checked file is linked without replacing another producer's
// destination. Root handles retain the original generation throughout copying.
func (output *copiedHLSVariantOutput) publishFirst(current func() bool) error {
	file, _, err := copiedHLSOpenFile(output.stage, "segment-00000.m4s", 64<<20)
	if err != nil {
		return err
	}
	defer file.Close()
	name := filepath.Base(output.directory) + ".pending"
	pending, err := output.media.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errCopiedHLSIndex
	}
	owned, err := pending.Stat()
	if err != nil {
		_ = pending.Close()
		return errCopiedHLSIndex
	}
	defer func() {
		_ = pending.Close()
		copiedHLSRemoveOwned(output.media, name, owned)
	}()
	if err := copyCopiedHLSPending(output.ctx, file, pending); err != nil {
		return err
	}
	if !current() || !output.certifiedPending(name, owned) {
		return errCopiedHLSIndex
	}
	linkErr := output.media.Link(name, "segment-00000.m4s")
	if linkErr != nil && !errors.Is(linkErr, os.ErrExist) {
		return errCopiedHLSIndex
	}
	return output.finishPublishedFirst(linkErr, current, owned)
}

func copyCopiedHLSPending(ctx context.Context, source, pending *os.File) error {
	size, err := io.Copy(pending, copiedHLSContextReader{ctx, io.LimitReader(source, (64<<20)+1)})
	if closeErr := pending.Close(); err == nil {
		err = closeErr
	}
	if err != nil || size <= 0 || size > 64<<20 {
		return errCopiedHLSIndex
	}
	return nil
}

func (output *copiedHLSVariantOutput) finishPublishedFirst(linkErr error, current func() bool, owned os.FileInfo) error {
	// A race winner is accepted only when independently certified; never remove
	// or replace that path on any subsequent failure.
	valid := current()
	hash, err := copiedHLSAssetHash(output.ctx, output.media, "segment-00000.m4s", 64<<20)
	if !valid || err != nil || hash != output.certificate.First {
		if linkErr == nil {
			copiedHLSRemoveOwned(output.media, "segment-00000.m4s", owned)
		}
		return errCopiedHLSIndex
	}
	return nil
}

func (output *copiedHLSVariantOutput) certifiedPending(name string, owned os.FileInfo) bool {
	hash, err := copiedHLSAssetHash(output.ctx, output.media, name, 64<<20)
	info, statErr := output.media.Lstat(name)
	return err == nil && statErr == nil && hash == output.certificate.First && os.SameFile(owned, info) && output.ctx.Err() == nil
}

func copiedHLSRemoveStageRoot(root *os.Root, directory string) {
	parent, err := os.OpenRoot(filepath.Dir(directory))
	if err != nil {
		return
	}
	defer parent.Close()
	name := filepath.Base(directory)
	held, heldErr := root.Stat(".")
	canonical, err := parent.Lstat(name)
	if heldErr == nil && err == nil && canonical.IsDir() && os.SameFile(held, canonical) {
		_ = parent.Remove(name) // Empty exclusive operation stage after worker/monitor join.
	}
}

func copiedHLSSettleStage(root *os.Root, directory string) {
	file, err := root.Open(".")
	if err != nil {
		return
	}
	defer file.Close()
	deadline := time.Now().Add(time.Second)
	for removed := 0; removed < 4096 && time.Now().Before(deadline); {
		entries, err := file.ReadDir(32)
		for _, entry := range entries {
			_ = root.Remove(entry.Name()) // Files only; never traverse unexpected directories.
		}
		removed += len(entries)
		if err != nil || len(entries) == 0 {
			break
		}
	}
	copiedHLSRemoveStageRoot(root, directory)
}

func (output *copiedHLSProbeOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > 8<<10 {
		return 0, errCopiedHLSIndex
	}
	return output.Buffer.Write(data)
}

func (manager *hlsManager) ensureCopiedHLSClock(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string) error {
	timeline, err := manager.copiedHLSClockPending(ctx, directory, policy)
	if err != nil || !timeline {
		return err
	}

	ctx, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer release()
	indexed, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
	if err != nil || indexed.Clock != nil {
		return err
	}
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return errCopiedHLSIndex
	}
	for _, rendition := range strings.Split(string(master), "\n") {
		if !hlsFile(rendition) || !strings.HasSuffix(rendition, "/index.m3u8") {
			continue
		}
		return manager.bindCopiedHLSClock(ctx, item, recipe, directory, rendition, policy, indexed)
	}
	return errCopiedHLSIndex
}

func indexedCopiedHLSSegmentArguments(arguments []string, timeline *copiedHLSTimeline) []string {
	if timeline != nil {
		for number := range arguments {
			if arguments[number] == "-hls_time" && number+1 < len(arguments) {
				arguments[number+1] = "0.1"
			}
		}
	}
	return arguments
}

func (manager *hlsManager) bindCopiedHLSClock(ctx context.Context, item library.Item, recipe hlsRecipe, directory, rendition, policy string, timeline *copiedHLSTimeline) (result error) {
	observation := ctx
	defer func() {
		copiedHLSClockRejected(observation, recipe.mode, result)
	}()
	if !validCopiedHLSClockInput(ctx, rendition, timeline) {
		return errCopiedHLSIndex
	}
	ctx, release, err := manager.copiedHLSClockAdmission(ctx)
	if err != nil {
		return err
	}
	defer release()
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	name := filepath.Dir(rendition)
	media, err := root.OpenRoot(name)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer media.Close()
	before, err := copiedHLSClockPendingSnapshot(ctx, root, media, policy, timeline)
	if err != nil {
		return err
	}
	var clock float64
	var probeErr error
	bound := *timeline
	if timeline.AudioOrigin != nil {
		clock = 0
		probeErr = manager.verifyCopiedAACVideoClock(ctx, media)
		if probeErr == nil {
			bound.AudioOrigin, probeErr = manager.measureCopiedAACOrigin(ctx, item, recipe, policy, timeline, media)
		}
	} else {
		clock, probeErr = manager.measureCopiedHLSClock(ctx, media)
	}
	if probeErr != nil || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil || !manager.sameCopiedHLSClockGeneration(ctx, directory, name, root, media, before) {
		return errCopiedHLSIndex
	}
	bound.Clock = &clock
	if !validCopiedAACOrigin(&bound) {
		return errCopiedHLSIndex
	}
	if err := manager.commitCopiedHLSClock(ctx, item, recipe, directory, name, policy, root, media, before, &bound); err != nil {
		return err
	}
	*timeline = bound
	return nil
}

func (manager *hlsManager) commitCopiedHLSClock(ctx context.Context, item library.Item, recipe hlsRecipe, directory, name, policy string, root, media *os.Root, before copiedHLSClockGeneration, bound *copiedHLSTimeline) error {
	data, err := json.Marshal(bound)
	if err != nil || len(data) > maximumCopiedHLSTimelineBytes {
		return errCopiedHLSIndex
	}
	certificate, err := json.Marshal(copiedHLSClockCertificate{Version: copiedHLSCertificateVersion(bound), Rendition: name, Timeline: sha256.Sum256(data), Initialization: before.hashes[2], First: before.hashes[3]})
	if err != nil || writeCopiedHLSMetadata(root, ".copy-clock", certificate) != nil || !manager.sameCopiedHLSClockGeneration(ctx, directory, name, root, media, before) || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return errCopiedHLSIndex
	}
	if err := writeCopiedHLSMetadata(root, ".copy-timeline", data); err != nil {
		return err
	}
	if ctx.Err() != nil || !manager.copiedHLSCanonicalGeneration(directory, name, root, media) {
		return errCopiedHLSIndex
	}
	return nil
}

func copiedHLSClockRejected(ctx context.Context, mode string, result error) {
	if result != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "HLS copied clock rejected", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", mode, "failure_class", "generation-or-assets")
	}
}

func (manager *hlsManager) copiedHLSClockPending(ctx context.Context, directory, policy string) (bool, error) {
	timeline, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
	if err != nil {
		if manager.copiedHLSTimelinePresent(directory) {
			return false, errCopiedHLSIndex
		}
		return false, nil // Ordinary cold streams have no indexed strategy.
	}
	return timeline.Clock == nil, nil
}
