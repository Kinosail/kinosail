package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

var errHLSIdentityChanged = errors.New("HLS source or playback policy changed")

func (manager *hlsManager) ensureHLSJob(ctx context.Context, item library.Item, key string, options transcodeSettings, recipe hlsRecipe) (*hlsJob, error) {
	for {
		manager.mu.Lock()
		if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
			manager.mu.Unlock()
			return nil, err
		}
		job := manager.jobs[key]
		if job != nil && job.cachePolicy != options.Cache {
			replaceHLSIdentity(ctx, job, recipe)
			manager.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-job.done:
			}
			continue
		}
		if job == nil {
			//nolint:gosec // Key is a scanned ID plus a validated recipe.
			if err := os.RemoveAll(filepath.Join(manager.cache, key)); err != nil {
				manager.mu.Unlock()
				return nil, err
			}
			jobContext, cancel := context.WithCancelCause(manager.ctx)
			job = &hlsJob{done: make(chan struct{}), cancel: cancel, cachePolicy: options.Cache, requestID: requestActivityID(ctx), playbackSession: requestPlaybackSession(ctx)}
			manager.jobs[key] = job
			//nolint:contextcheck // The Server lifecycle owns shared output after this request ends.
			go manager.encode(jobContext, item, job, options, recipe)
		}
		manager.mu.Unlock()
		return job, nil
	}
}

// Called under manager.mu so one replacement produces one diagnostic.
func replaceHLSIdentity(ctx context.Context, job *hlsJob, recipe hlsRecipe) {
	if job.replacing {
		return
	}
	job.replacing = true
	job.cancel(errHLSIdentityChanged)
	slog.InfoContext(ctx, "HLS stream identity changed", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", recipe.mode)
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
	options.Cache += ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=7"
	if err := playback.ValidateHLSSource(item.Path, options.Cache); err != nil {
		return transcodeSettings{}, err
	}
	return options, nil
}

// Recheck the complete settings, subtitle, and source snapshot under manager.mu.
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
