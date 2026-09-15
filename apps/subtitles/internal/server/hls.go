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
	"github.com/MikeO7/kinosail/packages/transcodehardware"
	"github.com/MikeO7/kinosail/packages/workload"
)

type hlsJob struct {
	done            chan struct{}
	err             error
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

func (manager *hlsManager) prepare(ctx context.Context, item library.Item, recipe hlsRecipe) error { //nolint:cyclop // Final cache, active job, and restart recovery are distinct playback states.
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
	options, err := manager.settings.transcodingFor(recipe.codec)
	if err != nil {
		return err
	}
	if recipe.subtitlePath != "" {
		options.Cache += ":subtitle=" + sourceVersion(recipe.subtitlePath)
	}
	options.Cache += ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=7"
	if cacheFresh(playlist, item.Path, options.Cache) {
		return nil
	}
	manager.mu.Lock()
	job := manager.jobs[key]
	if job == nil {
		//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
		if err := os.RemoveAll(filepath.Join(manager.cache, key)); err != nil {
			manager.mu.Unlock()
			return err
		}
		job = &hlsJob{done: make(chan struct{}), requestID: requestActivityID(ctx), playbackSession: requestPlaybackSession(ctx)}
		manager.jobs[key] = job
		//nolint:contextcheck // Encoding uses the Server lifecycle so a disconnected request does not destroy shared output.
		go manager.encode(item, job, options, recipe)
	}
	manager.mu.Unlock()

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

func (manager *hlsManager) encode(item library.Item, job *hlsJob, options transcodeSettings, recipe hlsRecipe) {
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
		job.err = manager.encodeVariants(item, directory, options, recipe)
		if job.err != nil && recipe.mode == "transcode" && options.Accelerator != "none" && manager.ctx.Err() == nil && transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
			options = playback.SourceTranscoding(options, mediaFactsFor(item, manager.probe.inspect(manager.ctx, item)), sharedHLSRecipe(recipe))
			candidates := manager.settings.hardware.Recovery(options)
			if options.HardwareDecode {
				manager.settings.hardware.RecordDecodeFailure(options)
			} else {
				manager.settings.hardware.RecordFailure(options)
			}
			for _, fallback := range candidates {
				if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil {
					break
				}
				if manager.ctx.Err() != nil {
					break
				}
				if err := os.RemoveAll(directory); err != nil {
					job.err = err
					break
				}
				if err := os.MkdirAll(directory, 0o700); err != nil {
					job.err = err
					break
				}
				softwareFallback = true
				job.err = manager.encodeVariants(item, directory, fallback, recipe)
				if job.err == nil || !transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
					break
				}
				manager.settings.hardware.RecordFailure(fallback)
			}
		}
		if job.err != nil {
			job.err = newHLSDiagnosticError(job.err, hlsDiagnostic(job.err, item.Path, directory), item.Path, directory)
			slog.Error("HLS transcode failed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", softwareFallback, "duration_ms", time.Since(started).Milliseconds(), "error", hlsDiagnostic(job.err))
			//nolint:gosec // G703: key is a scanned hexadecimal ID plus a validated track number.
			if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err != nil {
				_ = os.RemoveAll(directory)
			}
		} else {
			slog.Info("HLS transcode completed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", softwareFallback, "duration_ms", time.Since(started).Milliseconds())
		}
	}
	manager.mu.Lock()
	delete(manager.jobs, key)
	close(job.done)
	manager.mu.Unlock()
}

func (manager *hlsManager) encodeVariants(item library.Item, directory string, options transcodeSettings, recipe hlsRecipe) error {
	ctx, cancel := context.WithCancel(manager.ctx)
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
		results := make(chan error, 1)
		go func() {
			results <- manager.encodeVariant(ctx, item, directory, quality.Label, strconv.Itoa(quality.Width), strconv.FormatInt((quality.Bitrate-128_000)/1000, 10)+"k", "128k", facts.Duration, options, recipe, window, start)
		}()
		return publishVariants(ctx, item.Path, directory, options.Cache, hlsCodecs(facts, recipe, options.Codec), []PlaybackQuality{quality}, results, 1, false)
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
	results := make(chan error, 1)
	go func() {
		results <- manager.encodePresentation(ctx, item, directory, options, window, qualities, audioBitrate, start)
	}()
	return publishVariants(ctx, item.Path, directory, options.Cache, hlsCodecs(facts, recipe, options.Codec), qualities, results, 1, true)
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
