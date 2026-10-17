package catalog

import (
	"context"
	"reflect"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type Decoration struct {
	Apply       func([]library.Item) []library.Item
	Enrich      func(context.Context, []library.Item) []library.Item
	Enriches    func(library.Item) bool
	Background  bool
	StartupWait time.Duration
}

const ManagedStartupWait = 5 * time.Second

type enrichedItem struct {
	raw, value library.Item
}

func (index *Index) commit(ctx context.Context, raw []library.Item, err error, always bool) error {
	if err != nil {
		index.mu.Lock()
		index.err = err
		kept := index.ready
		index.mu.Unlock()
		logScanFailure(err, kept)
		return err
	}
	if !index.background {
		index.enrichPending(ctx, raw, len(raw), make(map[string]bool))
	}
	index.composeMu.Lock()
	index.raw = raw
	index.pruneEnriched()
	items := index.compose(raw)
	index.mu.Lock()
	index.err, index.ready, index.scanned = nil, true, time.Now()
	index.mu.Unlock()
	index.publishComposed(items, always, true)
	index.signalEnrichment()
	return nil
}

func (index *Index) Redecorate() {
	index.composeMu.Lock()
	index.publishComposed(index.compose(index.raw), false, true)
}

func (index *Index) publishComposed(items []library.Item, always, analyze bool) {
	snapshot := index.current.Load()
	if always || !sameItems(items, snapshot.Items()) {
		index.mu.Lock()
		snapshot = index.publish(items)
		index.mu.Unlock()
	}
	index.mu.RLock()
	analyzers := append([]func([]library.Item){}, index.analyzers...)
	index.mu.RUnlock()
	index.analyzeMu.Lock()
	index.composeMu.Unlock()
	defer index.analyzeMu.Unlock()
	if !analyze || index.analyzed == snapshot.Generation() {
		return
	}
	index.analyzed = snapshot.Generation()
	for _, analyze := range analyzers {
		analyze(snapshot.Items())
	}
}

func (index *Index) publish(items []library.Item) *library.Snapshot {
	generation := uint64(1)
	if current := index.current.Load(); current != nil {
		generation = current.Generation() + 1
	}
	snapshot := library.NewSnapshot(items, generation)
	index.current.Store(snapshot)
	index.titleOrders = nil
	index.titleOrderVersion++
	return snapshot
}

func (index *Index) compose(raw []library.Item) []library.Item {
	items := append([]library.Item(nil), raw...)
	for position := range items {
		if enriched, found := index.enriched[items[position].ID]; found && reflect.DeepEqual(enriched.raw, items[position]) {
			items[position] = enriched.value
		}
	}
	index.mu.RLock()
	decorate := index.decorate
	index.mu.RUnlock()
	if decorate != nil {
		items = decorate(items)
	}
	return items
}

func (index *Index) pruneEnriched() {
	if len(index.enriched) == 0 {
		return
	}
	present := make(map[string]enrichedItem, len(index.enriched))
	for _, item := range index.raw {
		if enriched, found := index.enriched[item.ID]; found {
			present[item.ID] = enriched
		}
	}
	index.enriched = present
}

func sameItems(left, right []library.Item) bool {
	if len(left) != len(right) {
		return false
	}
	for position := range left {
		if !reflect.DeepEqual(left[position], right[position]) {
			return false
		}
	}
	return true
}

func (index *Index) SetDecorator(decorate func([]library.Item) []library.Item) {
	index.mu.Lock()
	index.decorate = decorate
	index.mu.Unlock()
	index.Redecorate()
}

func (index *Index) AddAnalyzer(analyze func([]library.Item)) {
	index.analyzeMu.Lock()
	defer index.analyzeMu.Unlock()
	index.mu.Lock()
	index.analyzers = append(index.analyzers, analyze)
	index.mu.Unlock()
	analyze(index.current.Load().Items())
}

func (index *Index) Status() (int, time.Time, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	return index.current.Load().Len(), index.scanned, index.err
}
