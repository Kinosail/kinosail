package markers

import (
	"context"
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func (analyzer *Analyzer) analyzeBatch(ctx context.Context, items []library.Item, targets map[string]bool) error {
	groups := groupMarkerItems(items)
	titles := make([][]library.Item, 0, len(groups.seasons)+len(groups.movies))
	for _, group := range []map[string][]library.Item{groups.seasons, groups.movies} {
		for _, items := range group {
			titles = append(titles, items)
		}
	}
	if len(titles) == 0 {
		return nil
	}
	jobs, failures := make(chan int), make([]error, len(titles))
	var workers sync.WaitGroup
	for range min(len(titles), max(1, min(2, runtime.GOMAXPROCS(0)-1))) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for position := range jobs {
				failures[position] = analyzer.analyzeGroup(ctx, titles[position], targets)
			}
		}()
	}
dispatch:
	for position := range titles {
		select {
		case jobs <- position:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, failure := range failures {
		if failure != nil {
			return failure
		}
	}
	return nil
}

func (analyzer *Analyzer) analyzeGroup(ctx context.Context, items []library.Item, targets map[string]bool) error {
	analyzer.mu.RLock()
	before := make(map[string]Record, len(items))
	for _, item := range items {
		before[item.ID] = analyzer.records[item.ID]
	}
	staged := &Analyzer{
		cache: analyzer.cache, ffmpeg: analyzer.ffmpeg, tool: analyzer.tool, threads: analyzer.threads, acquire: analyzer.acquire,
		probe: analyzer.probe, extract: analyzer.extract, extractVisual: analyzer.extractVisual,
		extractCredits: analyzer.extractCredits, records: make(map[string]Record), targets: targets,
	}
	analyzer.mu.RUnlock()
	for _, item := range items {
		if item.Kind == "video" {
			staged.records[item.ID] = analyzedRecord(item, before[item.ID], nil)
		}
	}
	if err := staged.analyze(ctx, items); err != nil {
		return err
	}
	analyzer.mu.Lock()
	defer analyzer.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	updates := make(map[string]Record)
	for _, item := range items {
		if item.Kind != "video" || !staged.targeted(item) || staged.deferred[item.ID] {
			continue
		}
		current := analyzer.records[item.ID]
		if current.Revision != "" && current.Revision != before[item.ID].Revision && current.Revision != mediaRevision(item) {
			continue
		}
		record := analyzedRecord(item, current, staged.records[item.ID].Markers)
		if staged.failed[item.ID] {
			record = failedRecord(item, current)
		}
		updates[item.ID] = record
	}
	if err := analyzer.persistLocked(updates); err != nil {
		return ErrPersistence
	}
	if len(staged.failed) > 0 || len(staged.deferred) > 0 {
		slog.Warn("playback marker analysis skipped titles", "operation", "markers.analyze", "outcome", "partial", "failed", len(staged.failed), "deferred", len(staged.deferred), "titles", len(items))
	}
	return nil
}

func failedRecord(item library.Item, current Record) Record {
	record := analyzedRecord(item, current, nil)
	if current.Revision == record.Revision {
		record.Markers = current.Markers
	}
	record.Failed = time.Now().Unix()
	return record
}

func analyzedRecord(item library.Item, current Record, detected []Marker) Record {
	record := Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion, Suppressed: current.suppressed(item)}
	if current.Revision == record.Revision {
		for _, marker := range current.Markers {
			if marker.Source == "manual" {
				record.Markers = append(record.Markers, marker)
			}
		}
	}
	for _, marker := range detected {
		if marker.Source != "manual" && !slices.Contains(record.Suppressed, marker.Type) && !hasMarkerSource(record.Markers, marker.Type, "manual") {
			record.Markers = append(record.Markers, marker)
		}
	}
	return record
}
