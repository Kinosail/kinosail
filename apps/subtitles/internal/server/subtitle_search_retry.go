package server

import (
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func (ledger *subtitleLedger) automaticSearchReady(key string, item library.Item, now time.Time) bool {
	record, found, err := ledger.search(key)
	return err == nil && (!found || !record.matchesMedia(item) || now.Unix() >= record.NextAt)
}

func (record subtitleSearchRecord) matchesMedia(item library.Item) bool {
	return record.MediaSize == item.Size && record.MediaModified == subtitleMediaModified(item)
}

func subtitleMediaModified(item library.Item) int64 {
	if item.Added.Before(time.Unix(0, 0)) {
		return 0
	}
	return item.Added.UnixMicro()
}

func (ledger *subtitleLedger) noteSearch(key, outcome, safeError string, item library.Item, now time.Time) {
	previous, found, _ := ledger.search(key)
	attempts := 0
	if found && previous.matchesMedia(item) && outcome != "installed" {
		attempts = min(previous.Attempts+1, 1000)
	}
	delay := 24 * time.Hour
	switch outcome {
	case "provider-error":
		delay = min(time.Duration(1<<min(attempts, 6))*time.Minute, time.Hour)
	case "no-result":
		delays := []time.Duration{6 * time.Hour, 24 * time.Hour, 72 * time.Hour, 7 * 24 * time.Hour}
		delay = delays[min(attempts, len(delays)-1)]
	}
	_ = ledger.storeSearch(key, subtitleSearchRecord{Outcome: outcome, Attempts: attempts, CheckedAt: now.Unix(), NextAt: now.Add(delay).Unix(), SafeError: safeError, MediaSize: item.Size, MediaModified: subtitleMediaModified(item)})
}

func (ledger *subtitleLedger) clearSearches() {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.err != nil {
		return
	}
	previous := ledger.state.Searches
	ledger.state.Searches = make(map[string]subtitleSearchRecord)
	if err := ledger.save(); err != nil {
		ledger.state.Searches = previous
	}
}
