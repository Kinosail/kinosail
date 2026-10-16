package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleLedgerReloadsAfterLargeLibraryWrites(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	ledger := openTestSubtitleLedger(t, directory)
	now := time.Now()
	const pairs = 6400
	for position := range pairs {
		itemID := fmt.Sprintf("%016x", position+1)
		record := largeLibraryRecord(itemID, now)
		if err := ledger.store(subtitleRecordKey(itemID, "en"), record); err != nil {
			t.Fatalf("store pair %d: %v", position, err)
		}
		search := subtitleSearchRecord{Outcome: "installed", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix(), MediaSize: 1 << 32, MediaModified: now.UnixMicro()}
		if err := ledger.storeSearch(subtitleSearchKey(itemID, "en", "standard"), search); err != nil {
			t.Fatalf("store search %d: %v", position, err)
		}
	}
	reloaded := openTestSubtitleLedger(t, directory)
	if reloaded.err != nil {
		t.Fatalf("restart rejected the ledger it wrote: %v", reloaded.err)
	}
	if record, found, err := reloaded.record(subtitleRecordKey(fmt.Sprintf("%016x", pairs), "en")); err != nil || !found || record.Score != 90 {
		t.Fatalf("reloaded record = %#v, %v, %v", record, found, err)
	}
	if !reloaded.takeSubDLDownload(now) {
		t.Fatal("reloaded ledger refused a SubDL download")
	}
	extra := subtitleRecord{Fingerprint: subtitleFingerprint([]byte("extra")), Source: "external", Score: 100, CheckedAt: now.Unix()}
	if err := reloaded.store(subtitleRecordKey("ffffffffffffffff", "en"), extra); err != nil {
		t.Fatalf("store after restart: %v", err)
	}
}

func largeLibraryRecord(itemID string, now time.Time) subtitleRecord {
	return subtitleRecord{Fingerprint: subtitleFingerprint([]byte(itemID)), OriginalFingerprint: subtitleFingerprint([]byte("original" + itemID)), Source: "opensubtitles", Score: 90, CheckedAt: now.Unix(), InstalledAt: now.Unix(), Managed: true, Size: 64000, Modified: now.UnixNano(), ReleaseMatch: 0.75, TimingEvidence: "audio", Synchronization: "linear", Backup: true, BackupRole: "translation", BackupFingerprint: subtitleFingerprint([]byte("backup" + itemID)), BackupOriginalFingerprint: subtitleFingerprint([]byte("backup-original" + itemID)), Cleanup: []string{"hearing-impaired", "credits"}}
}

func TestSubtitleLedgerRecoversOversizedLegacyFile(t *testing.T) {
	t.Parallel()
	directory, now := t.TempDir(), time.Now()
	state := emptySubtitleLedgerState()
	for position := range 9000 {
		itemID := fmt.Sprintf("%016x", position+1)
		state.Records[subtitleRecordKey(itemID, "en")] = largeLibraryRecord(itemID, now)
		state.Searches[subtitleSearchKey(itemID, "en", "standard")] = subtitleSearchRecord{Outcome: "installed", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix()}
	}
	state.SubDLDay, state.SubDLDownloads = now.UTC().Format("2006-01-02"), 3
	encoded, err := json.Marshal(state)
	if err != nil || len(encoded) <= 4<<20 {
		t.Fatalf("legacy fixture = %d bytes, %v; want more than 4 MiB", len(encoded), err)
	}
	legacy := filepath.Join(directory, subtitleLedgerLegacyFile)
	if err := os.WriteFile(legacy, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if migrated := openTestSubtitleLedger(t, directory); migrated.err != nil || len(migrated.state.Records) != 9000 {
		t.Fatalf("migration = %d records, %v", len(migrated.state.Records), migrated.err)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy ledger remains after migration: %v", err)
	}
	reloaded := openTestSubtitleLedger(t, directory)
	if reloaded.err != nil || len(reloaded.state.Records) != 9000 || len(reloaded.state.Searches) != 9000 || reloaded.state.SubDLDownloads != 3 {
		t.Fatalf("reloaded = %d records, %d searches, %d downloads, %v", len(reloaded.state.Records), len(reloaded.state.Searches), reloaded.state.SubDLDownloads, reloaded.err)
	}
}

func TestSubtitleLedgerForgetsItemsMissingBeyondGrace(t *testing.T) {
	t.Parallel()
	directory, now := t.TempDir(), time.Now()
	ledger := openTestSubtitleLedger(t, directory)
	for _, itemID := range []string{"0000000000000001", "0000000000000002"} {
		if err := ledger.store(subtitleRecordKey(itemID, "en"), largeLibraryRecord(itemID, now)); err != nil {
			t.Fatal(err)
		}
		if err := ledger.storeSearch(subtitleSearchKey(itemID, "en", "standard"), subtitleSearchRecord{Outcome: "no-result", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix()}); err != nil {
			t.Fatal(err)
		}
	}
	present := []library.Item{{ID: "0000000000000001"}}
	ledger.forgetMissing(nil, now)
	ledger.forgetMissing(present, now.Add(subtitleLedgerMissingGrace-time.Minute))
	if len(ledger.state.Records) != 2 || len(ledger.state.Searches) != 2 {
		t.Fatalf("ledger forgot an item inside the grace period: %d records, %d searches", len(ledger.state.Records), len(ledger.state.Searches))
	}
	ledger.forgetMissing(present, now.Add(subtitleLedgerMissingGrace))
	reloaded := openTestSubtitleLedger(t, directory)
	if _, found := reloaded.state.Records[subtitleRecordKey("0000000000000002", "en")]; found || len(reloaded.state.Records) != 1 || len(reloaded.state.Searches) != 1 {
		t.Fatalf("missing item was not forgotten: %d records, %d searches", len(reloaded.state.Records), len(reloaded.state.Searches))
	}
}

func TestSubtitleLedgerReloadsAndPrunesRecordsBeyondDocumentLimit(t *testing.T) {
	t.Parallel()
	directory, now := t.TempDir(), time.Now()
	ledger := openTestSubtitleLedger(t, directory)
	const total = 26000
	live := make([]library.Item, 0, total)
	for start := 0; start < total; start += 2000 {
		records := make(map[string]any, 2000)
		for position := start; position < min(start+2000, total); position++ {
			itemID := fmt.Sprintf("%016x", position+1)
			records[subtitleRecordKey(itemID, "en")] = largeLibraryRecord(itemID, now)
			live = append(live, library.Item{ID: itemID})
		}
		if err := ledger.db.SaveEntries(subtitleLedgerRecordsDocument, records, nil); err != nil {
			t.Fatalf("store records from %d: %v", start, err)
		}
	}
	missing := live[total-1].ID
	if err := ledger.db.SaveEntry(subtitleLedgerSearchesDocument, subtitleSearchKey(missing, "en", "standard"), subtitleSearchRecord{Outcome: "no-result", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if data, found, err := ledger.db.Load(subtitleLedgerRecordsDocument); err != nil || !found || len(data) <= database.DocumentSizeLimit {
		t.Fatalf("records document = %d bytes, %v, %v; want more than %d", len(data), found, err, database.DocumentSizeLimit)
	}
	reloaded := openTestSubtitleLedger(t, directory)
	if reloaded.err != nil || len(reloaded.state.Records) != total || len(reloaded.state.Searches) != 1 {
		t.Fatalf("restart = %d records, %d searches, %v", len(reloaded.state.Records), len(reloaded.state.Searches), reloaded.err)
	}
	present := live[:total-1]
	reloaded.forgetMissing(present, now)
	reloaded.forgetMissing(present, now.Add(subtitleLedgerMissingGrace))
	pruned := openTestSubtitleLedger(t, directory)
	if _, found := pruned.state.Records[subtitleRecordKey(missing, "en")]; found || pruned.err != nil || len(pruned.state.Records) != total-1 || len(pruned.state.Searches) != 0 {
		t.Fatalf("prune after restart = %d records, %d searches, %v", len(pruned.state.Records), len(pruned.state.Searches), pruned.err)
	}
}
