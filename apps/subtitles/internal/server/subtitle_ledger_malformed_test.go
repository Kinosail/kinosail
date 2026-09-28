package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleLedgerRejectsMalformedPersistedStateAndStopsDownloads(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	path := filepath.Join(data, "subtitle_acquisitions.json")
	invalid := `{"version":1,"records":{"bad":{"fingerprint":"x"}}}`
	if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := newSubtitleLedger(data)
	if ledger.err == nil || ledger.takeSubDLDownload(time.Now()) {
		t.Fatalf("invalid ledger = %v, download allowed", ledger.err)
	}
	ledger.clearSearches()
	if contents, err := os.ReadFile(path); err != nil || string(contents) != invalid {
		t.Fatalf("invalid state changed: %v, %q", err, contents)
	}
}

func TestSubtitleLedgerRejectsInvalidMediaVersionWithoutChangingState(t *testing.T) {
	t.Parallel()
	for _, invalid := range []string{`"media_size":-1`, `"media_modified":-1`, `"media_size":"large"`} {
		t.Run(invalid, func(t *testing.T) {
			data := t.TempDir()
			path := filepath.Join(data, "subtitle_acquisitions.json")
			contents := fmt.Sprintf(`{"version":2,"records":{},"searches":{"0123456789abcdef:en:standard":{"outcome":"no-result","attempts":1,"checked_at":%d,"next_at":%d,%s}}}`, time.Now().Unix(), time.Now().Add(time.Hour).Unix(), invalid)
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			ledger := newSubtitleLedger(data)
			if ledger.err == nil || ledger.automaticSearchReady(subtitleSearchKey("0123456789abcdef", "en", "standard"), library.Item{}, time.Now()) || ledger.takeSubDLDownload(time.Now()) {
				t.Fatalf("invalid state accepted: %v", ledger.err)
			}
			stored, err := os.ReadFile(path)
			if err != nil || string(stored) != contents {
				t.Fatalf("invalid state changed: %v, %q", err, stored)
			}
		})
	}
}
