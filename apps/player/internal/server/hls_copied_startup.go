package server

import (
	"context"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) copiedHLSVideo(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	if recipe.mode == "remux" {
		return true
	}
	return recipe.mode == "audio-transcode" && !recipe.dialogueBoost && !recipe.normalizeLoudness &&
		len(recipe.omitted) == 0 && item.Kind != "audio" && item.Kind != "audiobook" &&
		manager.probe.facts(ctx, item).Video.Codec == "h264"
}

func (manager *hlsManager) reusableCopiedHLS(ctx context.Context, item library.Item, directory, policy string, recipe hlsRecipe) bool {
	if ctx.Err() != nil || !cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, policy) && !seekCacheFresh(directory, item.Path, policy) {
		return false
	}
	if !manager.startupCompletionReusable(ctx, item, directory, policy) {
		return false
	}
	if !manager.copiedHLSVideo(ctx, item, recipe) || !manager.copiedHLSTimelinePresent(directory) {
		// Ordinary cold streams retain their pre-index cache and seek behavior.
		return true
	}
	timeline, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
	if err != nil {
		slog.WarnContext(ctx, "HLS copied cache rejected", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", recipe.mode, "failure_class", "invalid-timeline")
		return false
	}
	return timeline.Clock != nil && ctx.Err() == nil
}

func (manager *hlsManager) bindCopiedHLSTimeline(ctx context.Context, directory string, startNumber int) error {
	preparation, ok := ctx.Value(startupEncodingKey{}).(*startupEncoding)
	if !ok || preparation.timeline == nil || startNumber != 0 {
		return nil
	}
	return manager.writeCopiedHLSTimeline(directory, preparation.timeline)
}

func copiedHLSSeekArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	if timeline == nil {
		return arguments, nil
	}
	if number < 0 || number >= len(timeline.Keys) {
		return nil, errCopiedHLSIndex
	}
	if timeline.Presentation != nil || timeline.Strategy == copiedHLSPrerollStrategy {
		if number != 0 || !validCopiedHLSPendingProducer(timeline) {
			return nil, errCopiedHLSIndex
		}
		origin := float64(timeline.Presentation.RequestedMicros) / 1_000_000
		return append(arguments, "-copypriorss:v", "0", "-avoid_negative_ts", "disabled",
			"-output_ts_offset", copiedHLSTime(timeline.point(0)-origin)), nil
	}
	// FFmpeg can demux an earlier key for input -ss. Do not copy that preroll.
	arguments = append(arguments, "-copypriorss", "0")
	if number > 0 {
		if timeline.Clock == nil {
			return nil, errCopiedHLSIndex
		}
		start := timeline.point(number)
		floor := math.Floor(start*1_000_000) / 1_000_000
		offset := start - timeline.point(0) + *timeline.Clock - (start - floor)
		arguments = append(arguments, "-output_ts_offset", copiedHLSTime(offset))
		// The mux clock restores copied video DTS; audio already keeps its source
		// presentation offset. Avoid adding that clock twice at a refill boundary.
		clock := copiedHLSTime(*timeline.Clock)
		arguments = append(arguments, "-bsf:a", "setts=pts=PTS-"+clock+"/TB:dts=DTS-"+clock+"/TB")
	}
	return arguments, nil
}

func (manager *hlsManager) copiedPlaylistProjection(ctx context.Context, item library.Item, recipe hlsRecipe, directory, rendition, policy string) func([]byte) []byte {
	return func(manifest []byte) []byte {
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		timeline, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
		if err != nil && manager.copiedHLSTimelinePresent(directory) {
			return nil
		}
		if err == nil {
			result, valid := copiedHLSManifest(manifest, timeline)
			if valid {
				return result
			}
			return nil // A present index never falls back to an uncertified EOF.
		}
		if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
			// EOF correction reads generated media, not container format duration.
			manifest = manager.completedCopiedHLSProjection(ctx, filepath.Join(directory, rendition), policy, manifest)
		} else if err != nil {
			// Preserve the existing established-cadence projection for unindexed
			// cold caches; these cuts do not acquire an indexed-source certificate.
			manifest = completeHLSVOD(manifest, hlsPlaybackDuration(recipe, manager.probe.duration(ctx, item)))
		}
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		return manifest
	}
}

func (manager *hlsManager) copiedStartupProjection(item library.Item, recipe hlsRecipe, directory string) func([]byte) []byte {
	if manager.completeHEVCStartup(manager.ctx, item, recipe) {
		return func(manifest []byte) []byte {
			if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
				return manifest
			}
			return nil
		}
	}
	if !manager.copiedHLSVideo(manager.ctx, item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	return func(manifest []byte) []byte {
		timeline, err := manager.readCopiedHLSTimelineContext(manager.ctx, directory, options.Cache)
		if err != nil && manager.copiedHLSTimelinePresent(directory) {
			return nil
		}
		if err == nil {
			projected, valid := copiedHLSManifest(manifest, timeline)
			if valid {
				return projected
			}
			return nil
		}
		if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
			return manifest
		}
		return nil
	}
}

func projectHLSPlaylist(manifest []byte, duration float64, projection ...func([]byte) []byte) []byte {
	if len(projection) > 0 && projection[0] != nil {
		return projection[0](manifest)
	}
	return completeHLSVOD(manifest, duration)
}

func copiedHLSInputTime(value float64) string {
	return copiedHLSTime(math.Floor(value*1_000_000) / 1_000_000)
}

func (manager *hlsManager) recipePlaylistProjection(ctx context.Context, item library.Item, recipe hlsRecipe, key, name string) func([]byte) []byte {
	if filepath.Dir(name) == "." || filepath.Ext(name) != ".m3u8" || !manager.copiedHLSVideo(ctx, item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	return manager.copiedPlaylistProjection(ctx, item, recipe, filepath.Join(manager.cache, key), filepath.Dir(name), options.Cache)
}

func (manager *hlsManager) copiedHLSTimelinePresent(directory string) bool {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Lstat(".copy-timeline")
	return err == nil
}

func copiedHLSRendition(root *os.Root) (string, error) {
	master, err := copiedHLSCacheFile(root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil {
		return "", err
	}
	selected := ""
	for _, line := range strings.Split(string(master), "\n") {
		if hlsFile(line) && strings.HasSuffix(line, "/index.m3u8") {
			if selected != "" {
				return "", errCopiedHLSIndex
			}
			selected = filepath.Dir(line)
		}
	}
	if selected == "" {
		return "", errCopiedHLSIndex
	}
	return selected, nil
}

// The actual retained output selection controls the enclosing master writer.
type copiedHLSOutputDecisionKey struct{}

type copiedHLSOutputDecision struct {
	once  sync.Once
	ready chan bool
}

func (decision *copiedHLSOutputDecision) set(staged bool) {
	decision.once.Do(func() { decision.ready <- staged })
}

func (manager *hlsManager) publishCopiedHLSWorker(ctx context.Context, source, directory, policy string, quality PlaybackQuality, number int, encode func(context.Context) error) error {
	decision := &copiedHLSOutputDecision{ready: make(chan bool, 1)}
	ctx = context.WithValue(ctx, copiedHLSOutputDecisionKey{}, decision)
	results, stop := playback.StartHLSWorker(ctx, func(ctx context.Context) error {
		defer decision.set(true) // Early admission/selection failure never publishes a master.
		return encode(ctx)
	})
	defer stop() // Preserve the existing cancellation and worker join owner.
	if number > 0 {
		return <-results
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case staged := <-decision.ready:
		if !staged {
			return publishVariants(ctx, source, directory, policy, []PlaybackQuality{quality}, results, 1, false)
		}
	}
	result := <-results
	stop()
	if result == nil {
		hlsObservationFor(ctx).emit("media_ready", "")
	}
	return result
}
