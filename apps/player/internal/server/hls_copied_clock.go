package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSProbeOutput struct {
	bytes.Buffer
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

func (manager *hlsManager) measureCopiedHLSClock(ctx context.Context, root *os.Root) (float64, error) {
	if ctx.Err() != nil {
		return 0, errCopiedHLSIndex
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return 0, err
	}
	fragment, err := copiedHLSCacheFile(root, "segment-00000.m4s", 64<<20)
	if err != nil {
		return 0, err
	}
	output := copiedHLSProbeOutput{}
	//nolint:gosec // Probe is installation config; input is bounded, rooted generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "v:0",
		"-read_intervals", "%+#8", "-show_packets", "-show_entries", "packet=pts_time,flags", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil {
		return 0, errCopiedHLSIndex
	}
	return decodeCopiedHLSClock(output.Bytes())
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
		if result != nil && observation.Err() == nil {
			slog.WarnContext(observation, "HLS copied clock rejected", "request_id", requestActivityID(observation), "playback_session", requestPlaybackSession(observation), "mode", recipe.mode, "failure_class", "generation-or-assets")
		}
	}()
	if ctx.Err() != nil || !hlsFile(rendition) || !strings.HasSuffix(rendition, "/index.m3u8") || !validCopiedHLSTimeline(timeline) || timeline.Clock != nil {
		return errCopiedHLSIndex
	}
	if ctx.Value(copiedHLSMetadataKey{}) != manager {
		admitted, release, err := manager.copiedHLSMetadataAdmission(ctx)
		if err != nil {
			return err
		}
		defer release()
		ctx = admitted
	}
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
	before, err := copiedHLSClockSnapshot(ctx, root, media)
	data, marshalErr := json.Marshal(timeline)
	if err != nil || marshalErr != nil || before.hashes[0] != sha256.Sum256([]byte(policy)) || before.hashes[1] != sha256.Sum256(data) {
		return errCopiedHLSIndex
	}
	clock, probeErr := manager.measureCopiedHLSClock(ctx, media)
	if probeErr != nil || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil || !manager.sameCopiedHLSClockGeneration(ctx, directory, name, root, media, before) {
		return errCopiedHLSIndex
	}
	bound := *timeline
	bound.Clock = &clock
	data, err = json.Marshal(&bound)
	if err != nil || len(data) > maximumCopiedHLSTimelineBytes {
		return errCopiedHLSIndex
	}
	certificate, err := json.Marshal(copiedHLSClockCertificate{Version: 1, Rendition: name, Timeline: sha256.Sum256(data), Initialization: before.hashes[2], First: before.hashes[3]})
	if err != nil || writeCopiedHLSMetadata(root, ".copy-clock", certificate) != nil || !manager.sameCopiedHLSClockGeneration(ctx, directory, name, root, media, before) || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return errCopiedHLSIndex
	}
	if err := writeCopiedHLSMetadata(root, ".copy-timeline", data); err != nil {
		return err
	}
	if ctx.Err() != nil || !manager.copiedHLSCanonicalGeneration(directory, name, root, media) {
		return errCopiedHLSIndex
	}
	*timeline = bound
	return nil
}

func decodeCopiedHLSClock(data []byte) (float64, error) {
	var facts struct {
		Packets []struct {
			PTS   string `json:"pts_time"`
			Flags string `json:"flags"`
		} `json:"packets"`
	}
	if json.Unmarshal(data, &facts) != nil || len(facts.Packets) == 0 ||
		!strings.Contains(facts.Packets[0].Flags, "K") {
		return 0, errCopiedHLSIndex
	}
	clock, err := strconv.ParseFloat(facts.Packets[0].PTS, 64)
	if err != nil || math.IsNaN(clock) || clock < 0 || clock > 1 {
		return 0, errCopiedHLSIndex
	}
	return clock, nil
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
