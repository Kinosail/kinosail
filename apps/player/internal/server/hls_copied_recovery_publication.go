package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSVariantOutput struct {
	ctx, observation              context.Context
	cancel                        context.CancelFunc
	directory, playlist, original string
	root, media, stage            *os.Root
	timeline                      *copiedHLSTimeline
	certificate                   copiedHLSClockCertificate
	timelineData, certificateData []byte
	manifest, master              []byte
	initialization, physical      os.FileInfo
	accepted                      bool
	failed                        bool
}

func copiedHLSRemoveOwned(root *os.Root, name string, owned os.FileInfo) {
	if info, err := root.Lstat(name); err == nil && info.Mode().IsRegular() && os.SameFile(owned, info) {
		_ = root.Remove(name)
	}
}

func copiedHLSRemoveStageRoot(root *os.Root, directory string) {
	held, heldErr := root.Stat(".")
	canonical, err := os.Lstat(directory)
	if heldErr == nil && err == nil && canonical.IsDir() && os.SameFile(held, canonical) {
		_ = os.Remove(directory) // Empty exclusive operation stage after worker/monitor join.
	}
}

func (manager *hlsManager) prepareCopiedHLSOutput(ctx context.Context, directory, name, policy, mode string, number int) (string, string, *copiedHLSVariantOutput, error) {
	output, err := manager.copiedHLSVariantOutput(ctx, directory, name, policy, mode, number)
	if err != nil {
		return "", "", nil, err
	}
	return output.directory, output.playlist, output, nil
}

func (manager *hlsManager) copiedHLSVariantOutput(ctx context.Context, directory, name, policy, mode string, number int) (*copiedHLSVariantOutput, error) {
	output := &copiedHLSVariantOutput{ctx: ctx, observation: ctx, original: directory}
	if number == 0 && (mode == "remux" || mode == "audio-transcode") {
		if _, err := os.Lstat(filepath.Join(directory, ".copy-timeline")); !os.IsNotExist(err) {
			if err != nil || output.selectStage(manager, name, policy) != nil {
				output.failed = true
				output.close()
				return nil, errCopiedHLSIndex
			}
		}
	}
	if output.stage != nil {
		return output, nil
	}
	media, playlists, err := preparePresentationDirectories(directory, name, number)
	output.directory, output.playlist = media, filepath.Join(playlists, "index.m3u8")
	if err != nil {
		output.close()
	}
	return output, err
}

func (output *copiedHLSVariantOutput) selectStage(manager *hlsManager, name, policy string) error {
	root, err := manager.openCopiedHLSRoot(output.original)
	if err != nil {
		return errCopiedHLSIndex
	}
	output.root = root
	timeline, err := manager.readCopiedHLSTimelineRoot(output.ctx, output.original, policy, root)
	if err != nil {
		return err
	}
	if timeline.Clock == nil {
		return nil
	}
	output.timeline = timeline
	output.media, err = root.OpenRoot(name)
	if err != nil {
		return errCopiedHLSIndex
	}
	if _, err := output.media.Lstat("segment-00000.m4s"); !os.IsNotExist(err) {
		return errCopiedHLSIndex // A committed zero must never enter the legacy writer.
	}
	if err := output.snapshot(name); err != nil {
		return err
	}
	output.directory, err = os.MkdirTemp(manager.cache, ".copy-refill-")
	if err != nil {
		return errCopiedHLSIndex
	}
	output.stage, err = os.OpenRoot(output.directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	output.playlist = filepath.Join(output.directory, "index.m3u8")
	output.ctx, output.cancel = context.WithTimeout(output.ctx, 30*time.Second)
	return nil
}

func (output *copiedHLSVariantOutput) snapshot(name string) error {
	var err error
	output.timelineData, err = copiedHLSCacheFile(output.root, ".copy-timeline", maximumCopiedHLSTimelineBytes)
	bound, marshalErr := json.Marshal(output.timeline)
	if err != nil || marshalErr != nil || !bytes.Equal(bound, output.timelineData) {
		return errCopiedHLSIndex
	}
	output.certificateData, err = copiedHLSCacheFile(output.root, ".copy-clock", 4096)
	certificate, decodeErr := decodeCopiedHLSCertificate(output.certificateData, output.timelineData)
	if err != nil || decodeErr != nil || certificate.Rendition != name {
		return errCopiedHLSIndex
	}
	output.certificate = certificate
	output.manifest, err = copiedHLSCacheFile(output.media, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil {
		return err
	}
	output.master, err = copiedHLSCacheFile(output.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil {
		return err
	}
	output.initialization, err = output.media.Lstat("init.mp4")
	if err != nil {
		return err
	}
	output.physical, err = output.media.Lstat("index.m3u8")
	return err
}

func (output *copiedHLSVariantOutput) arguments() []string {
	if output.stage == nil {
		return nil
	}
	// The next source key closes zero. Keep a second GOP for interleaver/audio
	// lookahead; a single key instead runs through its certified genuine EOF.
	duration := output.timeline.segmentEnd(min(1, len(output.timeline.Keys)-1)) - output.timeline.point(0) + *output.timeline.Clock
	return []string{"-t", copiedHLSTime(duration)}
}

func (output *copiedHLSVariantOutput) run(command *exec.Cmd, private ...string) error {
	if output.stage == nil {
		return runHLSCommand(output.ctx, command, private...)
	}
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-output.ctx.Done():
				return
			case <-ticker.C:
				if !output.boundedStage() {
					output.cancel()
					return
				}
			}
		}
	}()
	err := runHLSCommand(output.ctx, command, append(private, output.directory)...)
	close(stop)
	<-joined // The command and monitor settle before any publication or cleanup.
	if err != nil || output.ctx.Err() != nil || !output.boundedStage() {
		return errCopiedHLSIndex
	}
	return nil
}

func (output *copiedHLSVariantOutput) boundedStage() bool {
	file, err := output.stage.Open(".")
	if err != nil {
		return false
	}
	defer file.Close()
	entries, err := file.ReadDir(9)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > 8 {
		return false
	}
	total := int64(0)
	for _, entry := range entries {
		limit := copiedHLSStagedFileLimit(entry.Name())
		info, err := output.stage.Lstat(entry.Name())
		if err != nil || !info.Mode().IsRegular() || limit == 0 || info.Size() > limit {
			return false
		}
		total += info.Size()
	}
	return total <= 128<<20
}

func copiedHLSStagedFileLimit(name string) int64 {
	name = strings.TrimSuffix(name, ".tmp")
	switch name {
	case "index.m3u8":
		return maximumCopiedHLSTimelineBytes
	case "init.mp4":
		return 2 << 20
	}
	if number, valid := hlsSegmentNumber(name); valid && number <= 2 {
		return 64 << 20
	}
	return 0
}

func (output *copiedHLSVariantOutput) publish(manager *hlsManager, ctx context.Context, item library.Item, recipe hlsRecipe, policy string) error {
	if output.stage == nil {
		return finalizePlaylist(output.playlist)
	}
	initialization, err := copiedHLSAssetHash(output.ctx, output.stage, "init.mp4", 2<<20)
	if err != nil || initialization != output.certificate.Initialization || !output.validPrefix() || !output.current(manager, ctx, item, recipe, policy) {
		return errCopiedHLSIndex
	}
	if err := output.publishFirst(func() bool { return output.current(manager, ctx, item, recipe, policy) }); err != nil {
		return err
	}
	output.accepted = true
	return nil
}

func (output *copiedHLSVariantOutput) validPrefix() bool {
	manifest, err := copiedHLSCacheFile(output.stage, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") || !bytes.Contains(manifest, []byte("#EXT-X-MAP:URI=\"init.mp4\"")) {
		return false
	}
	lines := strings.Split(string(manifest), "\n")
	for number, line := range lines {
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			length, err := strconv.ParseFloat(value, 64)
			return err == nil && number+1 < len(lines) && lines[number+1] == "segment-00000.m4s" && matchesCopiedHLSLength(output.timeline, 0, length)
		}
	}
	return false
}

func (output *copiedHLSVariantOutput) current(manager *hlsManager, ctx context.Context, item library.Item, recipe hlsRecipe, policy string) bool {
	if ctx.Err() != nil || output.ctx.Err() != nil || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return false
	}
	return output.currentMetadata(manager, ctx, policy) && output.currentAssets()
}

func (output *copiedHLSVariantOutput) currentMetadata(manager *hlsManager, ctx context.Context, policy string) bool {
	if !copiedHLSBoundMetadata(output.root, output.timelineData, output.certificateData, policy) || !manager.copiedHLSCanonicalGeneration(output.original, output.certificate.Rendition, output.root, output.media) {
		return false
	}
	return manager.verifyCopiedHLSCertificate(ctx, output.original, output.root, output.timelineData, output.timeline) == nil
}

func (output *copiedHLSVariantOutput) currentAssets() bool {
	manifest, err := copiedHLSCacheFile(output.media, "index.m3u8", maximumCopiedHLSTimelineBytes)
	physical, statErr := output.media.Lstat("index.m3u8")
	init, initErr := output.media.Lstat("init.mp4")
	master, masterErr := copiedHLSCacheFile(output.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || statErr != nil || initErr != nil || masterErr != nil {
		return false
	}
	return bytes.Equal(output.manifest, manifest) && bytes.Equal(output.master, master) && sameCopiedHLSFile(output.physical, physical) && sameCopiedHLSFile(output.initialization, init)
}

func (output *copiedHLSVariantOutput) close() {
	if output.cancel != nil {
		output.cancel()
	}
	if (output.failed || output.timeline != nil && !output.accepted) && output.observation.Err() == nil {
		slog.WarnContext(output.observation, "HLS copied refill rejected", "request_id", requestActivityID(output.observation), "playback_session", requestPlaybackSession(output.observation), "failure_class", "generation-assets-or-stage")
	}
	for _, root := range []*os.Root{output.stage, output.media, output.root} {
		if root == output.stage && root != nil {
			copiedHLSSettleStage(root, output.directory)
		}
		if root != nil {
			_ = root.Close()
		}
	}
}
