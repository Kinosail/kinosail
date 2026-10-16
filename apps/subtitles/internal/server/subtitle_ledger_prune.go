package server

import (
	"log/slog"
	"strings"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
)

const subtitleLedgerMissingGrace = 7 * 24 * time.Hour

func (ledger *subtitleLedger) forgetMissing(items []library.Item, now time.Time) {
	live := make(map[string]struct{}, len(items))
	for _, item := range items {
		live[item.ID] = struct{}{}
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.err != nil {
		return
	}
	absent, expired := make(map[string]time.Time), make(map[string]bool)
	note := func(key string) {
		itemID, _, _ := strings.Cut(key, ":")
		if _, found := live[itemID]; found {
			return
		}
		if _, seen := absent[itemID]; seen {
			return
		}
		since, found := ledger.missing[itemID]
		if !found {
			since = now
		}
		absent[itemID] = since
		expired[itemID] = now.Sub(since) >= subtitleLedgerMissingGrace
	}
	for key := range ledger.state.Records {
		note(key)
	}
	for key := range ledger.state.Searches {
		note(key)
	}
	ledger.missing = absent
	records, removedRecords := withoutSubtitleItems(ledger.state.Records, expired)
	searches, removedSearches := withoutSubtitleItems(ledger.state.Searches, expired)
	changes := make(map[string]database.EntryChanges, 2)
	if len(removedRecords) > 0 {
		changes[subtitleLedgerRecordsDocument] = database.EntryChanges{Delete: removedRecords}
	}
	if len(removedSearches) > 0 {
		changes[subtitleLedgerSearchesDocument] = database.EntryChanges{Delete: removedSearches}
	}
	if len(changes) == 0 || ledger.commit(subtitleLedgerRecordsDocument, database.Batch{Entries: changes}) != nil {
		return
	}
	ledger.state.Records, ledger.state.Searches = records, searches
	slog.Info("subtitle acquisition state pruned", "records", len(removedRecords), "searches", len(removedSearches))
}

func withoutSubtitleItems[T any](entries map[string]T, expired map[string]bool) (map[string]T, []string) {
	kept, removed := make(map[string]T, len(entries)), make([]string, 0)
	for key, value := range entries {
		itemID, _, _ := strings.Cut(key, ":")
		if expired[itemID] {
			removed = append(removed, key)
			continue
		}
		kept[key] = value
	}
	return kept, removed
}
