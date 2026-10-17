package server

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

const subtitlePreparationWorkers = 2

type subtitlePreparation struct {
	probe    *mediaProbe
	language func() string
	acquire  func(context.Context) (func(), error)
	waitIdle func(context.Context) error
	busy     func() bool
	started  func() <-chan struct{}
	workers  int
	mu       sync.Mutex
	prepared map[string]string
}

func (probe *mediaProbe) scheduleSubtitles(ctx context.Context, index *libraryIndex, settings *settingsStore, workloads *workload.Governor, maintenance *maintenanceManager) {
	if probe.ffmpeg == "" || probe.cacheDir == "" {
		return
	}
	preparation := newSubtitlePreparation(probe, settings.subtitleLanguage, workloads, maintenance)
	pending := make(chan []library.Item, 1)
	index.AddAnalyzer(func(items []library.Item) {
		select {
		case <-pending:
		default:
		}
		pending <- items
	})
	go preparation.run(ctx, pending)
}

func newSubtitlePreparation(probe *mediaProbe, language func() string, workloads *workload.Governor, maintenance *maintenanceManager) *subtitlePreparation {
	probe.core.ConfigureCache(probe.cacheDir)
	return &subtitlePreparation{
		probe: probe, language: language, prepared: make(map[string]string), workers: min(subtitlePreparationWorkers, workloads.Metrics().BackgroundCapacity-1),
		acquire:  func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) },
		waitIdle: maintenance.WaitIdle, busy: maintenance.Busy, started: maintenance.ForegroundStarted,
	}
}

func (preparation *subtitlePreparation) run(ctx context.Context, pending <-chan []library.Item) {
	for {
		select {
		case <-ctx.Done():
			return
		case items := <-pending:
			preparation.prepare(ctx, items)
		}
	}
}

func (preparation *subtitlePreparation) prepare(ctx context.Context, items []library.Item) {
	language := preparation.language()
	work := make(chan library.Item)
	var workers sync.WaitGroup
	for range max(1, preparation.workers) {
		workers.Go(func() { preparation.work(ctx, work, language) })
	}
	present := make(map[string]bool, len(items))
	for _, item := range items {
		present[item.ID] = true
		preparation.mu.Lock()
		done := preparation.prepared[item.ID] == subtitlePreparationKey(item, language)
		preparation.mu.Unlock()
		if item.Kind != "video" || done {
			continue
		}
		select {
		case work <- item:
		case <-ctx.Done():
		}
	}
	close(work)
	workers.Wait()
	preparation.mu.Lock()
	for id := range preparation.prepared {
		if !present[id] {
			delete(preparation.prepared, id)
		}
	}
	preparation.mu.Unlock()
}

func (preparation *subtitlePreparation) work(ctx context.Context, work <-chan library.Item, language string) {
	for item := range work {
		err := preparation.prepareItem(ctx, item, language)
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			continue
		}
		if err != nil {
			slog.Warn("background subtitle preparation failed", "operation", "subtitles.prepare", "error", hlsDiagnostic(err, item.Path, preparation.probe.cacheDir))
		}
		preparation.mu.Lock()
		preparation.prepared[item.ID] = subtitlePreparationKey(item, language)
		preparation.mu.Unlock()
	}
}

func subtitlePreparationKey(item library.Item, language string) string {
	return strconv.FormatInt(item.Size, 10) + ":" + strconv.FormatInt(item.Added.UnixNano(), 10) + ":" + language
}

func (preparation *subtitlePreparation) prepareItem(ctx context.Context, item library.Item, language string) error {
	if preparation.probe.subtitlePrepared(item, language) {
		return nil
	}
	for ctx.Err() == nil {
		if err := preparation.waitIdle(ctx); err != nil {
			return err
		}
		release, err := preparation.acquire(ctx)
		if err != nil {
			return err
		}
		work, cancel := context.WithTimeout(workload.WithBackgroundPriority(ctx), 5*time.Minute)
		finished := make(chan struct{})
		go preparation.cancelOnForeground(work, cancel, finished)
		err = preparation.probe.preparePreferredSubtitle(work, item, language)
		cancel()
		<-finished
		release()
		if !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return ctx.Err()
}

func (preparation *subtitlePreparation) cancelOnForeground(work context.Context, cancel context.CancelFunc, finished chan<- struct{}) {
	defer close(finished)
	started := preparation.started()
	if preparation.busy() {
		cancel()
		return
	}
	select {
	case <-work.Done():
	case <-started:
		cancel()
	}
}

func (probe *mediaProbe) subtitlePrepared(item library.Item, language string) bool {
	preferred, err := canonicalSubtitleLanguage(language)
	if err != nil || item.Kind != "video" {
		return err == nil
	}
	facts, found := probe.core.CachedFacts(item)
	if !found {
		return false
	}
	stream, selected := preferredTextStream(item, facts, preferred)
	return !selected || probe.core.HasEmbedded(item, stream, probe.cacheDir)
}

func preferredTextStream(item library.Item, facts probeResult, preferred string) (int, bool) {
	selected := defaultTextSubtitle(playbackSubtitles(item, facts, preferred, true), preferred, true)
	for _, track := range facts.SubtitleFacts {
		if !track.Text {
			continue
		}
		if selected == 0 {
			return track.SourceIndex, true
		}
		selected--
	}
	return 0, false
}

func (probe *mediaProbe) preparePreferredSubtitle(ctx context.Context, item library.Item, language string) error {
	preferred, err := canonicalSubtitleLanguage(language)
	if err != nil {
		return err
	}
	if item.Kind != "video" {
		return nil
	}
	if stream, selected := preferredTextStream(item, probe.core.Facts(ctx, item), preferred); selected {
		options := probe.embeddedOptions()
		options.AllTextStreams = true
		_, err := probe.core.Embedded(ctx, item, stream, options)
		return err
	}
	return ctx.Err()
}
