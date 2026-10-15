package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type hlsJob struct {
	lifecycle       context.Context
	observation     *hlsObservation
	preparation     *startupEncoding
	done            chan struct{}
	err             error
	cancel          context.CancelCauseFunc
	activity        chan struct{}
	startNumber     int
	requestID       string
	playbackSession string
	pages           map[string]bool
	cachePolicy     string
	replacing       bool
}

func (manager *hlsManager) ensureHLSJob(ctx context.Context, item library.Item, key string, options transcodeSettings, recipe hlsRecipe) (*hlsJob, error) {
	for {
		manager.mu.Lock()
		if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
			manager.mu.Unlock()
			return nil, err
		}
		job := manager.jobs[key]
		if startupJobStopping(job) {
			manager.mu.Unlock()
			if err := waitForReplacedHLSJob(ctx, job); err != nil {
				return nil, err
			}
			return nil, errHLSIdentityChanged
		}
		if job != nil && job.cachePolicy != options.Cache {
			replaceHLSIdentity(ctx, job, recipe)
			manager.mu.Unlock()
			if err := waitForReplacedHLSJob(ctx, job); err != nil {
				return nil, err
			}
			continue
		}
		if job == nil {
			directory := filepath.Join(manager.cache, key)
			if cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, options.Cache) || seekCacheFresh(directory, item.Path, options.Cache) {
				if _, err := manager.readCopiedHLSTimelineContext(ctx, directory, options.Cache); err == nil {
					manager.mu.Unlock()
					return nil, errHLSIdentityChanged
				}
			}
			if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
				manager.mu.Unlock()
				return nil, err
			}
			//nolint:gosec // Key is a scanned ID plus a validated recipe.
			if err := os.RemoveAll(filepath.Join(manager.cache, key)); err != nil {
				manager.mu.Unlock()
				return nil, err
			}
			jobContext, created := manager.newHLSJob(ctx, 0)
			job = created
			job.cachePolicy = options.Cache
			manager.jobs[key] = job
			//nolint:contextcheck // The Server lifecycle owns shared output after this request ends.
			go manager.encode(jobContext, item, job, key, options, recipe, 0, false)
		}
		manager.adoptStartupJob(ctx, job, key)
		retainHLSPage(job, ctx)
		manager.mu.Unlock()
		return job, nil
	}
}

func replaceHLSIdentity(ctx context.Context, job *hlsJob, recipe hlsRecipe) {
	if job.replacing {
		return
	}
	job.replacing = true
	job.cancel(errHLSIdentityChanged)
	slog.InfoContext(ctx, "HLS stream identity changed", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", recipe.mode)
}

func waitForReplacedHLSJob(ctx context.Context, job *hlsJob) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-job.done:
		return nil
	}
}

func (manager *hlsManager) prepare(ctx context.Context, item library.Item, recipe hlsRecipe) error {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := manager.prepareAttempt(ctx, item, recipe)
		if !errors.Is(err, errHLSIdentityChanged) && !errors.Is(err, playback.ErrHLSSourceChanged) {
			return err
		}
	}
	return errHLSIdentityChanged
}

func (manager *hlsManager) hlsSettings(item library.Item, recipe hlsRecipe) (transcodeSettings, error) {
	options, err := manager.settings.transcodingFor(recipe.codec)
	if err != nil {
		return transcodeSettings{}, err
	}
	if recipe.subtitlePath != "" {
		options.Cache += ":subtitle=" + sourceVersion(recipe.subtitlePath)
	}
	if recipe.mode == "transcode" && item.Kind != "audio" && item.Kind != "audiobook" {
		options.Cache += ":transcode-clock=1"
	}
	if remainingAACOriginRecipe(item, recipe) {
		options.Cache += ":aac-origin=1"
	}
	if recipe.mode == "remux" || recipe.mode == "audio-transcode" {
		options.Cache += ":copied-origin=1"
		if recipe.offset > 0 {
			options.Cache += ":copied-resume=1"
		}
	}
	if recipe.mode == "audio-transcode" {
		options.Cache += ":copied-completion=1"
		if item.Kind != "audio" && item.Kind != "audiobook" {
			options.Cache += ":copied-audio-origin=3"
		}
	}
	options.Cache += ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=15"
	if err := playback.ValidateHLSSource(item.Path, options.Cache); err != nil {
		return transcodeSettings{}, err
	}
	return options, nil
}

func (manager *hlsManager) validateHLSPolicy(ctx context.Context, item library.Item, recipe hlsRecipe, expected string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := manager.hlsSettings(item, recipe)
	if err != nil {
		if errors.Is(err, playback.ErrHLSSourceChanged) {
			return errHLSIdentityChanged
		}
		return err
	}
	if current.Cache != expected {
		return errHLSIdentityChanged
	}
	return nil
}

func (job *hlsJob) coversSegment(rendition string, segment int) bool {
	if job == nil || segment < job.startNumber {
		return false
	}
	if segment <= job.startNumber+2 {
		return true
	}
	playlist := filepath.Join(rendition, "index.m3u8")
	if job.startNumber > 0 {
		playlist = filepath.Join(filepath.Dir(rendition), hlsSeekDirectory(job.startNumber), filepath.Base(rendition), "index.m3u8")
	}
	manifest, err := os.ReadFile(playlist) //nolint:gosec // Rendition and seek paths use generated cache names only.
	if err != nil {
		return false
	}
	latest := -1
	for _, line := range strings.Split(string(manifest), "\n") {
		if number, valid := hlsSegmentNumber(strings.TrimSpace(line)); valid {
			latest = max(latest, number)
		}
	}
	return latest >= job.startNumber && segment <= latest+2
}
