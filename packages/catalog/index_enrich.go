package catalog

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	enrichmentBatch           = 32
	enrichmentPublishInterval = 10 * time.Second
	enrichmentPublishCost     = 20
)

func (index *Index) signalEnrichment() {
	if index.enrich == nil || !index.background {
		return
	}
	index.enrichStart.Do(func() { go index.enrichLoop(index.lifecycle) })
	select {
	case index.enrichSignal <- struct{}{}:
	default:
	}
}

func (index *Index) enrichLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-index.enrichSignal:
		}
		index.enrichBackground(ctx)
	}
}

func (index *Index) enrichBackground(ctx context.Context) {
	attempted := make(map[string]bool)
	published, enriched, composed := time.Now(), false, time.Duration(0)
	for ctx.Err() == nil {
		release, err := index.acquire(ctx)
		if err != nil {
			return
		}
		index.composeMu.Lock()
		raw := index.raw
		index.composeMu.Unlock()
		progressed := index.enrichPending(ctx, raw, enrichmentBatch, attempted)
		release()
		if !progressed {
			break
		}
		enriched = true
		if time.Since(published) >= max(enrichmentPublishInterval, enrichmentPublishCost*composed) {
			started := time.Now()
			index.composeMu.Lock()
			index.publishComposed(index.compose(index.raw), false, false)
			published = time.Now()
			composed = published.Sub(started)
		}
	}
	if enriched && ctx.Err() == nil {
		index.composeMu.Lock()
		index.publishComposed(index.compose(index.raw), false, true)
		slog.Debug("Library enrichment completed", "operation", "library.enrich", "items", len(attempted))
	}
}

func (index *Index) enrichPending(ctx context.Context, raw []library.Item, limit int, attempted map[string]bool) bool {
	if index.enrich == nil || index.enriches == nil {
		return false
	}
	index.composeMu.Lock()
	pending := make([]library.Item, 0, min(limit, len(raw)))
	for _, item := range raw {
		if len(pending) == limit {
			break
		}
		if !index.enriches(item) || attempted[item.ID] {
			continue
		}
		if cached, found := index.enriched[item.ID]; found && reflect.DeepEqual(cached.raw, item) {
			continue
		}
		attempted[item.ID] = true
		pending = append(pending, item)
	}
	index.composeMu.Unlock()
	if len(pending) == 0 {
		return false
	}
	values := index.enrich(ctx, append([]library.Item(nil), pending...))
	if ctx.Err() != nil || len(values) != len(pending) {
		return false
	}
	index.composeMu.Lock()
	defer index.composeMu.Unlock()
	if index.enriched == nil {
		index.enriched = make(map[string]enrichedItem)
	}
	for position := range pending {
		index.enriched[pending[position].ID] = enrichedItem{raw: pending[position], value: values[position]}
	}
	return true
}
