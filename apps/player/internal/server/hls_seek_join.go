package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

type hlsSeekRequest struct {
	key, path    string
	segment      int
	indexed      bool
	options      transcodeSettings
	recipe, seek hlsRecipe
}

func (manager *hlsManager) joinOrSeekHLSJob(ctx context.Context, item library.Item, request hlsSeekRequest) (*hlsJob, *hlsJob, error) {
	if _, err := os.Stat(request.path); err == nil {
		return nil, nil, nil
	}
	manager.mu.Lock()
	job := manager.jobs[request.key]
	if startupJobStopping(job) {
		manager.mu.Unlock()
		return nil, job, nil
	}
	manager.adoptStartupJob(ctx, job, request.key)
	manager.mu.Unlock()
	if job.coversSegment(filepath.Dir(request.path), request.segment) {
		return job, nil, nil
	}
	if job != nil && ctx.Value(startupEncodingKey{}) != nil {
		return nil, nil, nil
	}
	if job != nil {
		job.cancel(errHLSSeekRestart)
		return nil, job, nil
	}
	return manager.startSeekHLSJob(ctx, item, request)
}

func (manager *hlsManager) startSeekHLSJob(ctx context.Context, item library.Item, request hlsSeekRequest) (*hlsJob, *hlsJob, error) {
	directory := filepath.Join(manager.cache, request.key)
	var recovery *transcodeHLSRecovery
	if request.recipe.mode == "transcode" {
		var err error
		recovery, err = manager.readTranscodeHLSRecovery(ctx, directory, request.options.Cache, filepath.Base(filepath.Dir(request.path)), request.segment)
		if err != nil {
			return nil, nil, err
		}
		if recovery != nil {
			recovery.original = request.recipe
		}
	}
	if err := writeAtomicFile(filepath.Join(directory, ".seekable"), []byte(request.options.Cache)); err != nil {
		return nil, nil, err
	}
	manager.mu.Lock()
	jobContext, job := manager.newHLSJob(ctx, request.segment)
	if request.indexed {
		jobContext = context.WithValue(jobContext, copiedHLSRequiredKey{}, true)
	}
	if recovery != nil {
		jobContext = context.WithValue(jobContext, transcodeHLSRecoveryKey{}, recovery)
	}
	job.cachePolicy = request.options.Cache
	manager.jobs[request.key] = job
	manager.mu.Unlock()
	go manager.encode(jobContext, item, job, request.key, request.options, request.seek, request.segment, true)
	return job, nil, nil
}
