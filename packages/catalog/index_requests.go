package catalog

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

const maximumPendingChanges = 4096

type refreshRequest struct {
	full, verify bool
	changed      map[string]struct{}
}

func (index *Index) RequestRefresh() {
	index.pendingMu.Lock()
	index.pending.full = true
	index.pendingMu.Unlock()
	index.signalRefresh()
}

func (index *Index) RequestChanges(paths []string) {
	if len(paths) == 0 {
		return
	}
	index.pendingMu.Lock()
	if index.pending.changed == nil {
		index.pending.changed = make(map[string]struct{}, len(paths))
	}
	for _, path := range paths {
		index.pending.changed[path] = struct{}{}
	}
	if len(index.pending.changed) > maximumPendingChanges {
		index.pending.full, index.pending.changed = true, nil
	}
	index.pendingMu.Unlock()
	index.signalRefresh()
}

func (index *Index) requestVerify() {
	index.pendingMu.Lock()
	index.pending.verify = true
	index.pendingMu.Unlock()
	index.signalRefresh()
}

func (index *Index) signalRefresh() {
	select {
	case index.refreshes <- struct{}{}:
	default:
	}
}

func (index *Index) takeRequest() refreshRequest {
	index.pendingMu.Lock()
	defer index.pendingMu.Unlock()
	request := index.pending
	index.pending = refreshRequest{}
	return request
}

func (index *Index) runRefreshes(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-index.refreshes:
			index.serveRequest(ctx, index.takeRequest())
		}
	}
}

func (index *Index) serveRequest(ctx context.Context, request refreshRequest) {
	if request.full {
		if err := index.Refresh(ctx); err != nil {
			slog.Warn("background Library scan failed", "error", err)
			return
		}
		items, scanned, _ := index.Status()
		slog.Debug("background Library scan completed", "items", items, "scanned", scanned)
		return
	}
	changed := slices.Collect(maps.Keys(request.changed))
	if request.verify {
		changed = append(changed, slices.Collect(maps.Keys(index.changedDirectories(ctx, index.Roots())))...)
	}
	if len(changed) == 0 {
		return
	}
	if err := index.refreshChanged(ctx, changed); err != nil {
		slog.Warn("incremental Library scan failed", "operation", "library.rescan", "paths", len(changed), "error", err)
		return
	}
	items, _, _ := index.Status()
	slog.Debug("incremental Library scan completed", "operation", "library.rescan", "paths", len(changed), "items", items)
}

func (index *Index) changedDirectories(ctx context.Context, roots []ScanRoot) map[string]int64 {
	index.refreshMu.Lock()
	scanners := make([]*library.Scanner, 0, len(roots))
	for _, root := range roots {
		if scanner := index.scanners[root]; scanner != nil {
			scanners = append(scanners, scanner)
		}
	}
	index.refreshMu.Unlock()
	changed := make(map[string]int64)
	for _, scanner := range scanners {
		maps.Copy(changed, scanner.Changed(ctx))
	}
	return changed
}
