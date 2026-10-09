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

func (manager *hlsManager) prepareSegment(ctx context.Context, item library.Item, recipe hlsRecipe, name string) error { //nolint:cyclop,gocognit,funlen // Replacement and joining must stay atomic for concurrent segment requests.
	if err := ctx.Err(); err != nil {
		return err
	}
	segment, valid := hlsSegmentNumber(filepath.Base(name))
	if !valid {
		return errors.New("HLS segment is invalid")
	}
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), mediaFactsFor(item, manager.probe.facts(ctx, item)), item.Subtitles)
	if sourceErr != nil {
		return sourceErr
	}
	recipe = localHLSRecipe(resolved)
	key, directory := hlsRecipeKey(item.ID, recipe), filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	if err := manager.qualifyCopiedAAC(ctx, item, recipe, false); err != nil {
		return err
	}
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); err == nil {
		if err := manager.copiedAACCacheAsset(ctx, item, recipe, directory, name); err != nil {
			return err
		}
		return manager.remainingAACCacheAsset(ctx, item, recipe, name)
	}
	duration := manager.probe.duration(ctx, item)
	manifest, err := os.ReadFile(filepath.Join(filepath.Dir(path), "index.m3u8")) //nolint:gosec // The path passed the HLS file allowlist.
	if err != nil {
		return err
	}
	options, err := manager.seekSettings(item, recipe, directory)
	if err != nil {
		return err
	}
	offset, valid := hlsSegmentOffset(manifest, filepath.Base(name), duration)
	if timeline, mapErr := manager.readCopiedHLSTimeline(directory, options.Cache); mapErr == nil {
		valid = timeline.Clock != nil && segment < len(timeline.Keys)
		if valid {
			offset = timeline.point(segment) - timeline.point(0)
		}
	} else if copiedAACPolicyRequired(options.Cache) || manager.copiedHLSTimelinePresent(directory) {
		return errCopiedHLSIndex
	}
	if !valid || offset >= hlsPlaybackDuration(recipe, duration) {
		return errors.New("HLS segment is outside the playable duration")
	}
	seekRecipe := recipe
	seekRecipe.offset += offset
	seekRecipe.outputTime = offset
	if startupActualPlayback(ctx) {
		manager.startup.playback(key)
	}
	for {
		manager.mu.Lock()
		if err := ctx.Err(); err != nil {
			manager.mu.Unlock()
			return err
		}
		if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
			manager.mu.Unlock()
			return err
		}
		if _, err = os.Stat(path); err == nil {
			manager.mu.Unlock()
			return manager.copiedAACCacheAsset(ctx, item, recipe, directory, name)
		}
		job := manager.jobs[key]
		if startupJobStopping(job) {
			manager.mu.Unlock()
			if err := waitForReplacedHLSJob(ctx, job); err != nil {
				return err
			}
			continue
		}
		if job != nil && job.cachePolicy != options.Cache {
			replaceHLSIdentity(ctx, job, recipe)
			manager.mu.Unlock()
			if err := waitForReplacedHLSJob(ctx, job); err != nil {
				return err
			}
			continue
		}
		manager.adoptStartupJob(ctx, job, key)
		if job.coversSegment(filepath.Dir(path), segment) {
			manager.mu.Unlock()
			return nil
		}
		if job != nil && ctx.Value(startupEncodingKey{}) != nil {
			// Preparation cannot replace a foreground seek or any shared encoder.
			manager.mu.Unlock()
			return nil
		}
		if job != nil {
			job.cancel(errHLSSeekRestart)
			manager.mu.Unlock()
			if err := waitForReplacedHLSJob(ctx, job); err != nil {
				return err
			}
			continue
		}
		if err = writeAtomicFile(filepath.Join(directory, ".seekable"), []byte(options.Cache)); err != nil {
			manager.mu.Unlock()
			return err
		}
		jobContext, created := manager.newHLSJob(ctx, segment)
		job = created
		job.cachePolicy = options.Cache
		manager.jobs[key] = job
		manager.mu.Unlock()
		//nolint:contextcheck // Encoding uses the Server lifecycle so a disconnected segment request does not destroy shared output.
		go manager.encode(context.WithValue(jobContext, copiedAACWorkerKey{}, &copiedAACWorkerIdentity{key: key, recipe: recipe, policy: options.Cache, job: job}), item, job, key, options, seekRecipe, segment, true)
		return nil
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
