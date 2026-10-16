package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

const hlsIdleTimeout = 45 * time.Second

var (
	errHLSEncodeComplete  = errors.New("HLS producer completed")
	errHLSSeekRestart     = errors.New("HLS transcode replaced for seek")
	errHLSInactive        = errors.New("HLS playback is inactive")
	errHLSIdentityChanged = errors.New("HLS source or playback policy changed")
)

func (manager *hlsManager) newHLSJob(request context.Context, startNumber int) (context.Context, *hlsJob) {
	ctx, cancel := context.WithCancelCause(manager.ctx)
	job := &hlsJob{lifecycle: ctx, done: make(chan struct{}), cancel: cancel, activity: make(chan struct{}, 1), startNumber: startNumber, requestID: requestActivityID(request), playbackSession: requestPlaybackSession(request)}
	retainHLSPage(job, request)
	job.observation = newHLSObservation(job.requestID, startNumber)
	ctx = context.WithValue(ctx, hlsObservationKey{}, job.observation)
	if preparation, ok := request.Value(startupEncodingKey{}).(*startupEncoding); ok {
		job.preparation = preparation
		ctx = context.WithValue(ctx, startupEncodingKey{}, preparation)
		go manager.watchStartupCancellation(ctx, request, job)
	}
	go watchHLSJob(ctx, job, hlsIdleTimeout)
	return ctx, job
}

func watchHLSJob(ctx context.Context, job *hlsJob, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-job.activity:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(timeout)
		case <-timer.C:
			job.cancel(errHLSInactive)
			return
		}
	}
}

func (manager *hlsManager) keepHLSAlive(key string, ctx context.Context) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if job := manager.jobs[key]; job != nil {
		retainHLSPage(job, ctx)
		select {
		case job.activity <- struct{}{}:
		default:
		}
	}
}

func (manager *hlsManager) keepHLSSessionAlive(session string) {
	if !validPlaybackSession(session) {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for _, job := range manager.jobs {
		if job.playbackSession == session {
			select {
			case job.activity <- struct{}{}:
			default:
			}
		}
	}
}

func (manager *hlsManager) stopHLSSession(session string) {
	if !validPlaybackSession(session) {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for _, job := range manager.jobs {
		if job.playbackSession == session {
			job.cancel(errHLSInactive)
		}
	}
}

func (manager *hlsManager) prepareSegment(ctx context.Context, item library.Item, recipe hlsRecipe, name string) error {
	_, err := manager.prepareSegmentJob(ctx, item, recipe, name)
	return err
}

func (manager *hlsManager) prepareSegmentJob(ctx context.Context, item library.Item, recipe hlsRecipe, name string) (*hlsJob, error) { //nolint:cyclop,gocognit,funlen // Segment validation precedes the keyed join or seek restart.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	segment, valid := hlsSegmentNumber(filepath.Base(name))
	if !valid {
		return nil, errors.New("HLS segment is invalid")
	}
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), mediaFactsFor(item, manager.probe.facts(ctx, item)), item.Subtitles)
	if sourceErr != nil {
		return nil, sourceErr
	}
	recipe = localHLSRecipe(resolved)
	key, directory := hlsRecipeKey(item.ID, recipe), filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); err == nil {
		return nil, manager.remainingAACCacheAsset(ctx, item, recipe, name)
	}
	duration := manager.probe.duration(ctx, item)
	manifest, err := os.ReadFile(filepath.Join(filepath.Dir(path), "index.m3u8"))
	if err != nil {
		return nil, err
	}
	options, err := manager.seekSettings(item, recipe, directory)
	if err != nil {
		return nil, err
	}
	offset, valid := hlsSegmentOffset(manifest, filepath.Base(name), duration)
	playable := hlsPlaybackDuration(recipe, duration)
	indexed := false
	if timeline, mapErr := manager.readCopiedHLSTimeline(directory, options.Cache); mapErr == nil {
		indexed = true
		valid = timeline.Clock != nil && segment < len(timeline.Keys)
		if valid {
			offset = timeline.point(segment) - timeline.point(0)
			playable = timeline.End - timeline.point(0)
		}
	} else if manager.copiedHLSTimelinePresent(directory) || manager.coldCopiedHLSNeedsIndex(ctx, item, recipe) {
		return nil, errCopiedHLSIndex
	}
	if !valid || offset >= playable {
		return nil, errors.New("HLS segment is outside the playable duration")
	}
	seekRecipe := recipe
	seekRecipe.offset += offset
	seekRecipe.outputTime = offset
	if startupActualPlayback(ctx) {
		manager.startup.playback(key)
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
			return nil, err
		}
		unlock, err := manager.lockHLSKey(ctx, key)
		if err != nil {
			return nil, err
		}
		job, retry, err := manager.joinOrSeekHLSJob(ctx, item, hlsSeekRequest{key: key, path: path, segment: segment, indexed: indexed, options: options, recipe: recipe, seek: seekRecipe})
		unlock()
		if retry == nil {
			return job, err
		}
		if err := waitForReplacedHLSJob(ctx, retry); err != nil {
			return nil, err
		}
	}
}

func (manager *hlsManager) seekSettings(item library.Item, recipe hlsRecipe, directory string) (transcodeSettings, error) {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return transcodeSettings{}, err
	}
	master, readErr := os.ReadFile(filepath.Join(directory, "index.m3u8"))
	if readErr != nil || !strings.Contains(string(master), "#KINOSAIL-TRANSCODER:"+options.Cache+"\n") {
		return transcodeSettings{}, errors.New("playback settings changed; start a new compatible stream")
	}
	return options, nil
}

type hlsEncodeOutcome struct {
	softwareFallback bool
	superseded       bool
	inactive         bool
	preserve         bool
}

func (manager *hlsManager) encode(ctx context.Context, item library.Item, job *hlsJob, key string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) {
	started := time.Now()
	complete := manager.shortHEVCNeedsCompletion(ctx, item, recipe)
	job.observation.queued(recipe.mode)
	directory := filepath.Join(manager.cache, key)
	manager.prepareHLSEncodeDirectory(job, directory, preserve)
	outcome := hlsEncodeOutcome{preserve: preserve}
	if job.err == nil {
		outcome = manager.runHLSEncode(ctx, item, job, directory, options, recipe, startNumber, preserve)
		if complete && job.err == nil && !outcome.inactive && !outcome.superseded && ctx.Err() == nil {
			job.err = manager.publishShortHEVCCompletion(ctx, item, recipe, directory, options.Cache)
		}
	}
	manager.reportHLSEncode(job, item, directory, options, recipe, outcome, started)
	manager.finishHLSEncode(job, key, directory, startNumber, outcome.preserve)
}

func (manager *hlsManager) runHLSEncode(ctx context.Context, item library.Item, job *hlsJob, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) hlsEncodeOutcome {
	outcome := hlsEncodeOutcome{preserve: preserve}
	job.err = manager.encodeVariants(ctx, item, directory, options, recipe, startNumber)
	outcome.superseded = errors.Is(context.Cause(ctx), errHLSSeekRestart) || errors.Is(context.Cause(ctx), errHLSIdentityChanged)
	outcome.inactive = errors.Is(context.Cause(ctx), errHLSInactive)
	if outcome.superseded || outcome.inactive {
		job.err = nil
	}
	if outcome.inactive && !outcome.preserve {
		outcome.preserve = preserveInactiveHLS(job, directory, item.Path, options.Cache)
	}
	if manager.retrySoftwareHLSEncode(ctx, item, job, directory, options, recipe, startNumber, outcome.preserve) {
		outcome.softwareFallback = true
	}
	if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil && job.err != nil {
		outcome.preserve = true
	}
	return outcome
}

func preserveInactiveHLS(job *hlsJob, directory, source, cache string) bool {
	preserve := false
	if masterFresh(filepath.Join(directory, "index.m3u8"), source, cache) {
		job.err = writeAtomicFile(filepath.Join(directory, ".seekable"), []byte(cache))
		preserve = job.err == nil
	}
	if !preserve {
		_ = os.RemoveAll(directory)
	}
	return preserve
}
