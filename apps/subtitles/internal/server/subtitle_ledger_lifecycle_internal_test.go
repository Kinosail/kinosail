package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubtitleLedgerPersistsRecordsAndDailyLimits(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	ledger := newSubtitleLedger(directory)
	record := subtitleRecord{Fingerprint: subtitleFingerprint([]byte("subtitle")), Source: "subdl", Score: 90, CheckedAt: time.Now().Unix(), Managed: true}
	if err := ledger.store("0123456789abcdef:en", record); err != nil {
		t.Fatal(err)
	}
	reloaded := newSubtitleLedger(directory)
	stored, found, err := reloaded.record("0123456789abcdef:en")
	if err != nil || !found || stored.Fingerprint != record.Fingerprint {
		t.Fatalf("stored subtitle record = %#v, found %v, error %v", stored, found, err)
	}
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	for range subDLAutomaticDownloadLimit {
		if !reloaded.takeSubDLDownload(now) {
			t.Fatal("daily SubDL allowance ended early")
		}
	}
	if reloaded.takeSubDLDownload(now) {
		t.Fatal("daily SubDL allowance exceeded its limit")
	}
	if !reloaded.takeSubDLDownload(now.Add(24 * time.Hour)) {
		t.Fatal("SubDL allowance did not reset on a new day")
	}
}

func TestSubtitleLedgerRejectsInvalidStateAndRollsBackFailedWrites(t *testing.T) { //nolint:cyclop // One lifecycle proves state validation and rollback at each persistence failure.
	t.Parallel()
	ledger := newSubtitleLedger("")
	if err := ledger.store("0123456789abcdef:en", subtitleRecord{}); err == nil {
		t.Fatal("invalid subtitle record was accepted")
	}
	ledger.err = os.ErrPermission
	if ledger.takeSubDLDownload(time.Now()) {
		t.Fatal("unavailable ledger granted a SubDL download")
	}

	directory := t.TempDir()
	blocked := newSubtitleLedger(directory)
	blocker := filepath.Join(directory, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	blocked.path = filepath.Join(blocker, "subtitle_acquisitions.json")
	record := subtitleRecord{Fingerprint: subtitleFingerprint([]byte("subtitle")), Source: "external", Score: 100, CheckedAt: time.Now().Unix()}
	if err := blocked.store("0123456789abcdef:en", record); err == nil {
		t.Fatal("subtitle record write failure was accepted")
	}
	if _, found, _ := blocked.record("0123456789abcdef:en"); found {
		t.Fatal("failed subtitle record write changed memory")
	}
	beforeDay, beforeDownloads := blocked.state.SubDLDay, blocked.state.SubDLDownloads
	if blocked.takeSubDLDownload(time.Now()) || blocked.state.SubDLDay != beforeDay || blocked.state.SubDLDownloads != beforeDownloads {
		t.Fatal("failed allowance write changed memory")
	}
	blocked.state.Records["0123456789abcdef:en"] = record
	updated := record
	updated.Score = 90
	if err := blocked.store("0123456789abcdef:en", updated); err == nil || blocked.state.Records["0123456789abcdef:en"].Score != record.Score {
		t.Fatal("failed subtitle record update did not restore the previous record")
	}
	if err := newSubtitleLedger("").save(); err != nil {
		t.Fatalf("memory-only subtitle ledger save: %v", err)
	}
}

func TestSubtitleLedgerValidatesStrictCalendarDays(t *testing.T) {
	t.Parallel()
	for value, valid := range map[string]bool{
		"2026-08-30": true,
		"2026-02-30": false,
		"2026-8-30":  false,
		"":           false,
	} {
		if got := validSubtitleDay(value); got != valid {
			t.Fatalf("day %q validity = %v", value, got)
		}
	}
}
