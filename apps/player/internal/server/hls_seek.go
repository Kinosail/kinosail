package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

const hlsIdleTimeout = 45 * time.Second

var (
	errHLSSeekRestart = errors.New("HLS transcode replaced for seek")
	errHLSInactive    = errors.New("HLS playback is inactive")
)

func (manager *hlsManager) newHLSJob(request context.Context, startNumber int) (context.Context, *hlsJob) {
	ctx, cancel := context.WithCancelCause(manager.ctx)
	job := &hlsJob{done: make(chan struct{}), cancel: cancel, activity: make(chan struct{}, 1), startNumber: startNumber, requestID: requestActivityID(request), playbackSession: requestPlaybackSession(request)}
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

func (manager *hlsManager) keepHLSAlive(key string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if job := manager.jobs[key]; job != nil {
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
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	duration := manager.probe.duration(ctx, item)
	manifest, err := os.ReadFile(filepath.Join(filepath.Dir(path), "index.m3u8")) //nolint:gosec // The path passed the HLS file allowlist.
	if err != nil {
		return err
	}
	offset, valid := hlsSegmentOffset(manifest, filepath.Base(name), duration)
	if !valid || offset >= hlsPlaybackDuration(recipe, duration) {
		return errors.New("HLS segment is outside the playable duration")
	}
	seekRecipe := recipe
	seekRecipe.offset += offset
	seekRecipe.outputTime = offset
	options, err := manager.seekSettings(item, recipe, directory)
	if err != nil {
		return err
	}
	for {
		manager.mu.Lock()
		if _, err = os.Stat(path); err == nil {
			manager.mu.Unlock()
			return nil
		}
		job := manager.jobs[key]
		if job.coversSegment(filepath.Dir(path), segment) {
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
		manager.jobs[key] = job
		manager.mu.Unlock()
		//nolint:contextcheck // Encoding uses the Server lifecycle so a disconnected segment request does not destroy shared output.
		go manager.encode(jobContext, item, job, key, options, seekRecipe, segment, true)
		return nil
	}
}

func (manager *hlsManager) seekSettings(item library.Item, recipe hlsRecipe, directory string) (transcodeSettings, error) {
	options, err := manager.settings.transcodingFor(recipe.codec)
	if err != nil {
		return transcodeSettings{}, err
	}
	if recipe.subtitlePath != "" {
		options.Cache += ":subtitle=" + sourceVersion(recipe.subtitlePath)
	}
	options.Cache += ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=11"
	master, readErr := os.ReadFile(filepath.Join(directory, "index.m3u8"))
	if readErr != nil || !strings.Contains(string(master), "#KINOSAIL-TRANSCODER:"+options.Cache+"\n") {
		return transcodeSettings{}, errors.New("playback settings changed; start a new compatible stream")
	}
	return options, nil
}

func waitForReplacedHLSJob(ctx context.Context, job *hlsJob) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-job.done:
		return nil
	}
}

type hlsEncodeOutcome struct {
	softwareFallback bool
	superseded       bool
	inactive         bool
	preserve         bool
}

func (manager *hlsManager) encode(ctx context.Context, item library.Item, job *hlsJob, key string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) {
	started := time.Now()
	slog.Info("HLS transcode started", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator)
	directory := filepath.Join(manager.cache, key)
	manager.prepareHLSEncodeDirectory(job, directory, preserve)
	outcome := hlsEncodeOutcome{preserve: preserve}
	if job.err == nil {
		outcome = manager.runHLSEncode(ctx, item, job, directory, options, recipe, startNumber, preserve)
		manager.reportHLSEncode(job, item, directory, options, recipe, outcome, started)
	}
	manager.finishHLSEncode(job, key, directory, startNumber, outcome.preserve)
}

func (manager *hlsManager) prepareHLSEncodeDirectory(job *hlsJob, directory string, preserve bool) {
	if !preserve {
		job.err = os.RemoveAll(directory) //nolint:gosec // The key is a validated cache name.
	}
	if job.err == nil {
		job.err = os.MkdirAll(directory, 0o700) //nolint:gosec // The key is a validated cache name.
	}
}

func (manager *hlsManager) runHLSEncode(ctx context.Context, item library.Item, job *hlsJob, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) hlsEncodeOutcome {
	outcome := hlsEncodeOutcome{preserve: preserve}
	job.err = manager.encodeVariants(ctx, item, directory, options, recipe, startNumber)
	outcome.superseded = errors.Is(context.Cause(ctx), errHLSSeekRestart)
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
		_ = os.RemoveAll(directory) //nolint:gosec // The key is a validated cache name.
	}
	return preserve
}
