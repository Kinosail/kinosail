package markers

import (
	"context"
	"maps"
	"runtime"
	"slices"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
)

func (analyzer *Analyzer) analyzeBatch(ctx context.Context, items []library.Item) error { //nolint:cyclop,gocognit // Bounded group dispatch and ordered failures share one batch boundary.
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
				failures[position] = analyzer.analyzeGroup(ctx, titles[position])
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

// Publish each season or movie library atomically; unrelated groups can still finish.
func (analyzer *Analyzer) analyzeGroup(ctx context.Context, items []library.Item) error { //nolint:cyclop // Group staging, Owner edits, and atomic persistence form one publication boundary.
	analyzer.mu.RLock()
	before := maps.Clone(analyzer.records)
	staged := &Analyzer{
		cache: analyzer.cache, ffmpeg: analyzer.ffmpeg, tool: analyzer.tool,
		probe: analyzer.probe, extract: analyzer.extract, extractVisual: analyzer.extractVisual,
		extractCredits: analyzer.extractCredits, records: make(map[string]Record),
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
	next := maps.Clone(analyzer.records)
	for _, item := range items {
		if item.Kind != "video" {
			continue
		}
		current := next[item.ID]
		if current.Revision != "" && current.Revision != before[item.ID].Revision && current.Revision != mediaRevision(item) {
			continue // A newer media revision received an Owner edit during analysis.
		}
		next[item.ID] = analyzedRecord(item, current, staged.records[item.ID].Markers)
	}
	if analyzer.file != "" {
		if err := analyzer.persist(analyzer.file, next); err != nil {
			return ErrPersistence
		}
	}
	analyzer.records = next
	return nil
}

// Owner corrections made during extraction take precedence over generated results.
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
