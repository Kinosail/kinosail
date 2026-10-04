package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
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
	requestID       string
	playbackSession string
	cachePolicy     string
	replacing       bool
	cancel          context.CancelCauseFunc
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
	manager.cacheOps = playback.NewHLSCacheControl(cache, &manager.mu, func(name string) bool { return manager.jobs[name] != nil }, func() bool { return len(manager.jobs) > 0 }, hlsPolicy())
	return manager
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
	plan := playbackWithAutomaticSkip(mediaFactsFor(item, media), browserPlaybackCapabilities(manager.settings, nil), viewerPlaybackPolicy(currentViewer(request)), NetworkIntent{PreferCompatibility: true, AudioIndex: &track}, media.Markers, manager.settings.autoSkip())
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
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), mediaFactsFor(item, manager.probe.inspect(ctx, item)), item.Subtitles)
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
	if cacheFresh(playlist, item.Path, options.Cache) {
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
			return errHLSIdentityChanged
		case <-ticker.C:
		}
	}
}

func (manager *hlsManager) encode(ctx context.Context, item library.Item, job *hlsJob, options transcodeSettings, recipe hlsRecipe) {
	defer job.cancel(nil)
	started := time.Now()
	slog.Info("HLS transcode started", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator)
	softwareFallback := false
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(manager.cache, key)
	//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
	job.err = os.RemoveAll(directory)
	if job.err == nil {
		//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
		job.err = os.MkdirAll(directory, 0o700)
	}
	if job.err == nil {
		job.err = manager.encodeVariants(ctx, item, directory, options, recipe)
		if errors.Is(context.Cause(ctx), errHLSIdentityChanged) {
			job.err = nil
		}
		softwareFallback = manager.retrySoftwareHLSEncode(ctx, item, job, directory, options, recipe)
		if job.err != nil {
			job.err = newHLSDiagnosticError(job.err, hlsDiagnostic(job.err, item.Path, directory), item.Path, directory)
			slog.Error("HLS transcode failed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", softwareFallback, "duration_ms", time.Since(started).Milliseconds(), "error", hlsDiagnostic(job.err))
			//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
			if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err != nil {
				_ = os.RemoveAll(directory)
			}
		} else if !errors.Is(context.Cause(ctx), errHLSIdentityChanged) {
			slog.Info("HLS transcode completed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", softwareFallback, "duration_ms", time.Since(started).Milliseconds())
		}
	}
	manager.mu.Lock()
	delete(manager.jobs, key)
	close(job.done)
	manager.mu.Unlock()
}

func (manager *hlsManager) encodeVariants(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe) error {
	if err := playback.BindHLSSource(directory, item.Path, options.Cache); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	facts := mediaFactsFor(item, manager.probe.inspect(ctx, item))
	start, window := hlsWindowRecipe(recipe, facts.Duration)
	options = playback.SourceTranscoding(options, facts, sharedHLSRecipe(recipe))
	var err error
	options, err = manager.settings.hardware.ColorSettings(options)
	if err != nil {
		return err
	}
	window.subtitleTime = start
	if recipe.mode != "transcode" {
		quality := sourceQuality(facts, recipe.maxBitrate)
		results, stop := playback.StartHLSWorker(ctx, func(ctx context.Context) error {
			return manager.encodeVariant(ctx, item, directory, quality.Label, strconv.Itoa(quality.Width), strconv.FormatInt((quality.Bitrate-128_000)/1000, 10)+"k", "128k", facts.Duration, options, recipe, window, start)
		})
		defer stop()
		return publishVariants(ctx, item.Path, directory, options.Cache, []PlaybackQuality{quality}, results, 1, false)
	}
	width, height := playback.DisplayDimensions(facts.Video)
	width, height = playback.FitDimensions(width, height, minimumPositiveInt(1920, recipe.width), minimumPositiveInt(1080, recipe.height))
	qualities := adaptiveQualities(width, height, min(60, facts.Video.FrameRate), recipe.maxBitrate)
	capacity := manager.workloads.EncodingCapacity()
	if options.Accelerator != "none" {
		capacity = 1
	}
	if len(qualities) > capacity {
		qualities = qualities[len(qualities)-capacity:]
	}
	audioBitrate := int64(0)
	if len(facts.Audio) > 0 {
		audioBitrate = min(128_000, max(1_000, qualities[0].Bitrate/4))
	}
	results, stop := playback.StartHLSWorker(ctx, func(ctx context.Context) error {
		return manager.encodePresentation(ctx, item, directory, options, window, qualities, audioBitrate, start)
	})
	defer stop()
	return publishVariants(ctx, item.Path, directory, options.Cache, qualities, results, 1, true)
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
