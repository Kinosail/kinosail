package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

type hlsJob struct {
	done            chan struct{}
	err             error
	cancel          context.CancelCauseFunc
	activity        chan struct{}
	startNumber     int
	requestID       string
	playbackSession string
}

type hlsManager struct {
	ctx       context.Context
	cache     string
	ffmpeg    string
	index     *libraryIndex
	probe     *mediaProbe
	settings  *settingsStore
	mu        sync.Mutex
	jobs      map[string]*hlsJob
	workloads *workload.Governor
	cacheOps  playback.HLSCacheControl
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

func (manager *hlsManager) prepare(ctx context.Context, item library.Item, recipe hlsRecipe) error { //nolint:cyclop // Final cache, active job, and restart recovery are distinct playback states.
	if manager.cache == "" {
		return errors.New("compatible playback is not configured")
	}
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), mediaFactsFor(item, manager.probe.facts(ctx, item)), item.Subtitles)
	if sourceErr != nil {
		return sourceErr
	}
	recipe = localHLSRecipe(resolved)
	key := hlsRecipeKey(item.ID, recipe)
	playlist := filepath.Join(manager.cache, key, "index.m3u8")
	options, err := manager.settings.transcodingFor(recipe.codec)
	if err != nil {
		return err
	}
	if recipe.subtitlePath != "" {
		options.Cache += ":subtitle=" + sourceVersion(recipe.subtitlePath)
	}
	options.Cache += ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=11"
	if cacheFresh(playlist, item.Path, options.Cache) || seekCacheFresh(filepath.Dir(playlist), item.Path, options.Cache) {
		return nil
	}
	job, err := manager.ensureHLSJob(ctx, item, key, options, recipe)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if masterFresh(playlist, item.Path, options.Cache) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-job.done:
			if job.err != nil {
				return job.err
			}
			return manager.prepare(ctx, item, recipe)
		case <-ticker.C:
		}
	}
}

func (manager *hlsManager) encodeVariants(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int) error {
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
		results := make(chan error, 1)
		go func() {
			results <- manager.encodeVariant(ctx, item, directory, quality.Label, strconv.Itoa(quality.Width), strconv.FormatInt((quality.Bitrate-128_000)/1000, 10)+"k", "128k", facts.Duration, options, recipe, window, start, startNumber)
		}()
		if startNumber > 0 {
			return <-results
		}
		return publishVariants(ctx, item.Path, directory, options.Cache, hlsCodecs(facts, recipe, options.Codec), []PlaybackQuality{quality}, results, 1, false)
	}
	if err := playback.BindHLSEncoder(directory, options, startNumber > 0); err != nil {
		return err
	}
	qualities := manager.availableHLSQualities(facts, recipe, options)
	audioBitrate := int64(0)
	if len(facts.Audio) > 0 {
		audioBitrate = min(128_000, max(1_000, qualities[0].Bitrate/4))
	}
	results := make(chan error, 1)
	go func() {
		results <- manager.encodePresentation(ctx, item, directory, options, window, qualities, audioBitrate, start, startNumber)
	}()
	if startNumber > 0 {
		return <-results
	}
	return publishVariants(ctx, item.Path, directory, options.Cache, hlsCodecs(facts, recipe, options.Codec), qualities, results, 1, true)
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
	release, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return err
	}
	defer release()
	directory, playlistDirectory, err := preparePresentationDirectories(root, name, startNumber)
	if err != nil {
		return err
	}
	playlist := filepath.Join(playlistDirectory, "index.m3u8")
	input, video := videoArguments(options, width)
	arguments := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if startNumber > 0 {
		arguments = append(arguments, "-avoid_negative_ts", "disabled", "-max_delay", "5000000")
	}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	} else {
		arguments = append(arguments, "-readrate_initial_burst", "8", "-readrate", "1")
	}
	if start > 0 {
		arguments = append(arguments, "-ss", ffmpegSeconds(start))
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
	if startNumber > 0 {
		arguments = append(arguments, "-output_ts_offset", ffmpegSeconds(recipe.outputTime))
	}
	arguments = append(arguments, hlsSegmentArguments(recipe.mode, directory, playlist, startNumber)...)
	//nolint:gosec // G204: executable is installation config and input is found only by a Library scan.
	command := exec.CommandContext(ctx, manager.ffmpeg, arguments...)
	if err := runHLSCommand(command, item.Path, root); err != nil {
		return err
	}
	return finalizePlaylist(playlist)
}

func (manager *hlsManager) ensureHLSJob(ctx context.Context, item library.Item, key string, options transcodeSettings, recipe hlsRecipe) (*hlsJob, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	job := manager.jobs[key]
	if job == nil {
		//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
		if err := os.RemoveAll(filepath.Join(manager.cache, key)); err != nil {
			return nil, err
		}
		jobContext, created := manager.newHLSJob(ctx, 0)
		job = created
		manager.jobs[key] = job
		//nolint:contextcheck // Encoding uses the Server lifecycle so a disconnected request does not destroy shared output.
		go manager.encode(jobContext, item, job, key, options, recipe, 0, false)
	}
	return job, nil
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
