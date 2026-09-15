package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func (probe *mediaProbe) scheduleSubtitles(ctx context.Context, index *libraryIndex, settings *settingsStore, workloads *workload.Governor, maintenance *maintenanceManager) {
	if probe.ffmpeg == "" || probe.cacheDir == "" {
		return
	}
	pending := make(chan struct{}, 1)
	index.AddAnalyzer(func([]library.Item) {
		select {
		case pending <- struct{}{}:
		default:
		}
	})
	go probe.runSubtitlePreparation(ctx, index, settings, workloads, maintenance, pending)
}

func (probe *mediaProbe) runSubtitlePreparation(ctx context.Context, index *libraryIndex, settings *settingsStore, workloads *workload.Governor, maintenance *maintenanceManager, pending <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pending:
			if !probe.prepareSubtitleSnapshot(ctx, index, settings, workloads, maintenance) {
				return
			}
		}
	}
}

func (probe *mediaProbe) prepareSubtitleSnapshot(ctx context.Context, index *libraryIndex, settings *settingsStore, workloads *workload.Governor, maintenance *maintenanceManager) bool {
	items, err := index.Snapshot()
	if err != nil {
		return true
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return false
		}
		if item.Kind != "video" {
			continue
		}
		if err := probe.prepareIdleSubtitle(ctx, item, settings.subtitleLanguage(), workloads, maintenance); err != nil && ctx.Err() == nil {
			slog.Warn("background subtitle preparation failed", "error", hlsDiagnostic(err, item.Path, probe.cacheDir))
		}
	}
	return true
}

func (probe *mediaProbe) prepareIdleSubtitle(ctx context.Context, item library.Item, language string, workloads *workload.Governor, maintenance *maintenanceManager) error {
	for ctx.Err() == nil {
		if err := maintenance.WaitIdle(ctx); err != nil {
			return err
		}
		release, err := workloads.Acquire(ctx, workload.Background)
		if err != nil {
			return err
		}
		work, cancel := context.WithTimeout(ctx, 5*time.Minute)
		finished := make(chan struct{})
		go cancelBusySubtitle(work, cancel, maintenance, finished)
		err = probe.preparePreferredSubtitle(work, item, language)
		cancel()
		<-finished
		release()
		if !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return ctx.Err()
}

func cancelBusySubtitle(work context.Context, cancel context.CancelFunc, maintenance *maintenanceManager, finished chan<- struct{}) {
	defer close(finished)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if maintenance.Busy() {
			cancel()
			return
		}
		select {
		case <-work.Done():
			return
		case <-ticker.C:
		}
	}
}

func (probe *mediaProbe) preparePreferredSubtitle(ctx context.Context, item library.Item, language string) error {
	preferred, err := canonicalSubtitleLanguage(language)
	if err != nil {
		return err
	}
	if item.Kind != "video" {
		return nil
	}
	probe.core.ConfigureCache(probe.cacheDir)
	facts := probe.core.Facts(ctx, item)
	selected := defaultTextSubtitle(playbackSubtitles(item, facts, preferred, true), preferred, true)
	for _, track := range facts.SubtitleFacts {
		if !track.Text {
			continue
		}
		if selected == 0 {
			_, _, err := probe.extractEmbedded(ctx, item, track.SourceIndex)
			return err
		}
		selected--
	}
	return ctx.Err()
}
