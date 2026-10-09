package server

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

type hlsManager struct {
	startup        *startupPreparation
	copiedMetadata copiedHLSMetadata
	ctx            context.Context
	cache          string
	ffmpeg         string
	index          *libraryIndex
	probe          *mediaProbe
	settings       *settingsStore
	mu             sync.Mutex
	jobs           map[string]*hlsJob
	workloads      *workload.Governor
	cacheOps       playback.HLSCacheControl
}

func newHLS(ctx context.Context, cache, ffmpeg string, index *libraryIndex, probe *mediaProbe, settings *settingsStore, workloads *workload.Governor) *hlsManager {
	manager := &hlsManager{ctx: ctx, cache: cache, ffmpeg: ffmpeg, index: index, probe: probe, settings: settings, jobs: make(map[string]*hlsJob), workloads: workloads}
	manager.configureCache()
	return manager
}

func (manager *hlsManager) configureCache() {
	manager.cacheOps = playback.NewHLSCacheControl(manager.cache, &manager.mu, func(name string) bool { return manager.jobs[name] != nil }, func() bool { return len(manager.jobs) > 0 }, hlsPolicy())
}

func (manager *hlsManager) serve(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Legacy and planned HLS routes have distinct validation paths.
	playback.ServeHLS(writer, request, manager.hlsHandlerDependencies())
}

func (manager *hlsManager) hlsHandlerDependencies() playback.HLSHandlerDependencies {
	return playback.HLSHandlerDependencies{
		Policy: hlsPolicy(), Lookup: manager.hlsLookup, Allowed: hlsAllowed, Legacy: manager.legacyHLSRecipe,
		Duration: manager.hlsDuration, Serve: manager.serveSharedHLSRecipe, NotFound: localizedNotFound,
		Forbidden: hlsForbidden, InvalidSeek: hlsInvalidSeek,
	}
}

func (manager *hlsManager) hlsLookup(request *http.Request, id string) (library.Item, bool) {
	return visibleItem(request, manager.index, id)
}

func hlsAllowed(request *http.Request) bool {
	viewer := currentViewer(request)
	return viewer.Owner || viewer.Transcode
}

func (manager *hlsManager) legacyHLSRecipe(request *http.Request, item library.Item, track int) playback.HLSRecipe {
	media := manager.probe.inspect(request.Context(), item)
	intent := NetworkIntent{PreferCompatibility: true}
	if request.PathValue("track") != "" {
		intent.AudioIndex = &track
	}
	plan := playbackWithAutomaticSkip(mediaFactsFor(item, media), browserPlaybackCapabilities(manager.settings, nil), viewerPlaybackPolicy(currentViewer(request)), intent, media.Markers, manager.settings.autoSkip())
	return sharedHLSRecipe(recipeFor(plan))
}

func (manager *hlsManager) hlsDuration(request *http.Request, item library.Item) float64 {
	return manager.probe.duration(request.Context(), item)
}

func (manager *hlsManager) serveSharedHLSRecipe(writer http.ResponseWriter, request *http.Request, item library.Item, recipe playback.HLSRecipe, file string) {
	manager.serveRecipe(writer, request, item, localHLSRecipe(recipe), file)
}

func hlsForbidden(writer http.ResponseWriter, request *http.Request) {
	localizedError(writer, request, "transcoding is not allowed", http.StatusForbidden)
}

func hlsInvalidSeek(writer http.ResponseWriter, request *http.Request) {
	localizedError(writer, request, "seek is outside the playable duration", http.StatusBadRequest)
}

func (manager *hlsManager) prepareAttempt(ctx context.Context, item library.Item, recipe hlsRecipe) error { //nolint:cyclop // Final cache, active job, and restart recovery are distinct playback states.
	if manager.cache == "" {
		return errors.New("compatible playback is not configured")
	}
	facts := mediaFactsFor(item, manager.probe.facts(ctx, item))
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), facts, item.Subtitles)
	if sourceErr != nil {
		return sourceErr
	}
	recipe = localHLSRecipe(resolved)
	key := hlsRecipeKey(item.ID, recipe)
	playlist := filepath.Join(manager.cache, key, "index.m3u8")
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	if startupActualPlayback(ctx) {
		manager.startup.playback(key)
		manager.keepHLSAlive(key, ctx)
	}
	if manager.reusableCopiedHLS(ctx, item, filepath.Dir(playlist), options.Cache, recipe) {
		return refreshCachedVideoHLSMaster(playlist, facts, recipe, options.Cache)
	}
	job, err := manager.ensureHLSJob(ctx, item, key, options, recipe)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if masterFresh(playlist, item.Path, options.Cache) {
			return manager.ensureCopiedHLSClock(ctx, item, recipe, filepath.Dir(playlist), options.Cache)
		}
		select {
		case <-ctx.Done():
			return observedHLSReadinessError(ctx.Err(), job)
		case <-job.done:
			if job.err != nil {
				return observedHLSReadinessError(job.err, job)
			}
			return errHLSIdentityChanged
		case <-ticker.C:
		}
	}
}

func (manager *hlsManager) encodeVariants(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int) error {
	if err := playback.BindHLSSource(directory, item.Path, options.Cache); err != nil {
		return err
	}
	if err := manager.bindStartupCompletion(ctx, directory, options.Cache); err != nil {
		return err
	}
	if err := manager.bindCopiedHLSTimeline(ctx, directory, startNumber); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	facts := mediaFactsFor(item, manager.probe.facts(ctx, item))
	start, window := hlsWindowRecipe(recipe, facts.Duration)
	if item.Kind == "audio" || item.Kind == "audiobook" {
		return manager.encodeAudioVariant(ctx, item, directory, options, recipe, window, facts.Duration, start, startNumber)
	}
	options = playback.SourceTranscoding(options, facts, sharedHLSRecipe(recipe))
	var err error
	options, err = manager.settings.hardware.ColorSettings(options)
	if err != nil {
		return err
	}
	window.subtitleTime = start
	if recipe.mode != "transcode" {
		quality := sourceQuality(facts, recipe.maxBitrate)
		return manager.publishCopiedHLSWorker(ctx, item.Path, directory, options.Cache, quality, startNumber, func(ctx context.Context) error {
			return manager.encodeVariant(ctx, item, directory, quality.Label, strconv.Itoa(quality.Width), strconv.FormatInt((quality.Bitrate-128_000)/1000, 10)+"k", "128k", facts.Duration, options, recipe, window, start, startNumber)
		})
	}
	if err := playback.BindHLSEncoder(directory, options, startNumber > 0); err != nil {
		return err
	}
	qualities := manager.availableHLSQualities(facts, recipe, options)
	audioBitrate := int64(0)
	if len(facts.Audio) > 0 {
		audioBitrate = min(128_000, max(1_000, qualities[0].Bitrate/4))
	}
	results, stop := playback.StartHLSWorker(ctx, func(ctx context.Context) error {
		return manager.encodePresentation(ctx, item, directory, options, window, qualities, audioBitrate, start, startNumber)
	})
	defer stop()
	if startNumber > 0 {
		return <-results
	}
	return publishVariants(ctx, item.Path, directory, options.Cache, qualities, results, 1, true)
}

func hlsTranscodeQualities(facts MediaFacts, recipe hlsRecipe) []PlaybackQuality {
	width, height := playback.DisplayDimensions(facts.Video)
	width, height = playback.FitDimensions(width, height, minimumPositiveInt(1920, recipe.width), minimumPositiveInt(1080, recipe.height))
	qualities := adaptiveQualities(width, height, min(60, facts.Video.FrameRate), recipe.maxBitrate)
	if recipe.singleQuality && len(qualities) > 1 {
		qualities = qualities[len(qualities)-1:]
	}
	return qualities
}

func sourceQuality(facts MediaFacts, maximum int64) PlaybackQuality {
	width, height, bitrate := facts.Video.Width, facts.Video.Height, facts.Bitrate
	if width == 0 || height == 0 {
		width, height = 1920, 1080
	}
	if bitrate == 0 {
		bitrate = 6_128_000
	}
	if maximum > 0 && maximum < bitrate {
		bitrate = maximum
	}
	return PlaybackQuality{Label: qualityLabel(width, height), Width: width, Height: height, Bitrate: max(1, bitrate), FrameRate: facts.Video.FrameRate}
}

func (manager *hlsManager) encodeVariant(ctx context.Context, item library.Item, root, name, width, videoRate, audioRate string, duration float64, options transcodeSettings, sourceRecipe, recipe hlsRecipe, start float64, startNumber int) error { //nolint:cyclop,funlen // One FFmpeg command is assembled from the validated playback recipe.
	release, err := manager.acquireHLSEncode(ctx, 1, "", start, startNumber)
	if err != nil {
		return err
	}
	defer release()
	ctx, directory, playlist, output, err := manager.prepareCopiedHLSOutput(ctx, root, name, options.Cache, recipe.mode, startNumber)
	if err != nil {
		return err
	}
	defer output.close()
	timeline, _ := manager.readCopiedHLSTimeline(root, options.Cache)
	if !copiedHLSPendingProducerRecipe(timeline, sourceRecipe) {
		return errCopiedHLSIndex
	}
	if timeline != nil {
		if startNumber < 0 || startNumber >= len(timeline.Keys) {
			return errCopiedHLSIndex
		}
		start = timeline.point(startNumber)
	}
	var refill *remainingAudioOrigin
	if timeline == nil && startNumber > 0 && item.Kind == "audio" {
		refill = remainingAudioOriginRefill(mediaFactsFor(item, manager.probe.facts(ctx, item)), sourceRecipe, recipe, start, startNumber, audioRate)
	}
	input, video := videoArguments(options, width)
	arguments := startupInputArguments(ctx, []string{"-hide_banner", "-loglevel", "error", "-y"})
	if startNumber > 0 {
		arguments = append(arguments, "-avoid_negative_ts", "disabled", "-max_delay", "5000000")
	}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	}
	if start > 0 || timeline != nil {
		seek := ffmpegSeconds(start)
		if timeline != nil {
			seek = copiedHLSInputTime(start)
			arguments = append(arguments, "-seek_timestamp", "1")
		} else if refill != nil {
			seek = "0.000"
		}
		arguments = append(arguments, "-ss", seek)
	}
	arguments = append(arguments, "-i", item.Path)
	copyInput := "0"
	if recipe.mode != "transcode" && len(sourceRecipe.omitted) > 0 {
		arguments, copyInput, err = hlsSkipInput(arguments, directory, item.Path, duration, sourceRecipe)
		if err != nil {
			return err
		}
	}
	arguments, err = playback.HLSCodecArguments(playback.HLSCodecInput{AudioOnly: item.Kind == "audio" || item.Kind == "audiobook", Arguments: arguments, Video: video, Compatibility: videoCompatibilityArguments(options.Codec), ItemPath: item.Path, VideoRate: videoRate, AudioRate: audioRate, CopyInput: copyInput, Recipe: sharedHLSRecipe(recipe), Policy: hlsPolicy()})
	if err != nil {
		return err
	}
	if timeline == nil && startNumber > 0 {
		if refill != nil {
			arguments = append(arguments, "-output_ts_offset", refill.output, "-bsf:a", refill.drop)
		} else {
			arguments = append(arguments, "-output_ts_offset", ffmpegSeconds(recipe.outputTime))
		}
	}
	arguments, err = copiedHLSSeekArguments(arguments, timeline, startNumber)
	if err != nil {
		return err
	}
	arguments = append(arguments, output.arguments()...)
	arguments = append(arguments, indexedCopiedHLSSegmentArguments(hlsSegmentArguments(recipe.mode, directory, playlist, startNumber), timeline)...)
	//nolint:gosec // G204: executable is installation config and input is found only by a Library scan.
	command := exec.CommandContext(ctx, manager.ffmpeg, arguments...)
	if err := output.run(command, item.Path, root); err != nil {
		return err
	}
	return output.publish(manager, ctx, item, recipe, options.Cache)
}

func (manager *hlsManager) availableHLSQualities(facts MediaFacts, recipe hlsRecipe, options transcodeSettings) []PlaybackQuality {
	qualities := hlsTranscodeQualities(facts, recipe)
	capacity := manager.workloads.EncodingCapacity()
	if options.Accelerator != "none" {
		capacity = 1
	}
	if len(qualities) > capacity {
		qualities = qualities[len(qualities)-capacity:]
	}
	return qualities
}
