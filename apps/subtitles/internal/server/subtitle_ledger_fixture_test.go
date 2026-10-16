package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
)

func completeSubtitleRecord(path string, data []byte, record subtitleRecord) subtitleRecord {
	record.Fingerprint = subtitleFingerprint(data)
	if info, err := os.Stat(path); err == nil {
		record.Size, record.Modified = info.Size(), info.ModTime().UnixNano()
	}
	return record
}

func openTestSubtitleLedger(t testing.TB, directory string) *subtitleLedger {
	t.Helper()
	return newSubtitleLedger(directory, testStateDatabase(t, directory))
}

func testStateDatabase(t testing.TB, directory string) *database.Store {
	t.Helper()
	store, err := database.Open(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func persistedTestSubtitleLedger(directory string) ([]byte, error) {
	store, err := database.Open(directory, false)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ledger := newSubtitleLedger(directory, store)
	if ledger.err != nil {
		return nil, ledger.err
	}
	return json.Marshal(ledger.state)
}

func closedTestStateDatabase(t testing.TB) *database.Store {
	t.Helper()
	store := testStateDatabase(t, t.TempDir())
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return store
}
