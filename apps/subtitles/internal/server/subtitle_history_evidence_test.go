package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Persisted evidence can fail through unknown reasons, invalid types or bounds,
// incomplete score pairs, contradictory actions, or invented previous scores.
func TestSubtitleHistoryRejectsInvalidEvidenceWithoutSideEffects(t *testing.T) { //nolint:gocognit // One persistence boundary checks each malformed evidence case and unchanged state.
	t.Parallel()
	for name, evidence := range map[string]string{
		"unknown reason":                   `"reason":"better"`,
		"oversized reason":                 `"reason":"` + strings.Repeat("x", 513) + `"`,
		"malformed score":                  `"reason":"missing","score":"90","releaseMatch":1`,
		"negative score":                   `"reason":"missing","score":-1,"releaseMatch":1`,
		"high score":                       `"reason":"missing","score":101,"releaseMatch":1`,
		"high release":                     `"reason":"missing","score":90,"releaseMatch":1.1`,
		"negative release":                 `"reason":"missing","score":90,"releaseMatch":-0.1`,
		"missing score":                    `"reason":"missing","releaseMatch":1`,
		"missing release":                  `"reason":"missing","score":90`,
		"unexplained score":                `"score":90,"releaseMatch":1`,
		"wrong action":                     `"reason":"higher-score","score":90,"releaseMatch":1,"previousSource":"subdl","previousScore":70,"previousReleaseMatch":1`,
		"invented previous":                `"reason":"missing","score":90,"releaseMatch":1,"previousSource":"subdl","previousScore":70,"previousReleaseMatch":1`,
		"upgrade small gain":               `"reason":"higher-score","score":79,"releaseMatch":1,"previousSource":"subdl","previousScore":70,"previousReleaseMatch":1`,
		"upgrade unknown source":           `"reason":"higher-score","score":90,"releaseMatch":1,"previousSource":"remote","previousScore":70,"previousReleaseMatch":1`,
		"upgrade missing previous":         `"reason":"higher-score","score":90,"releaseMatch":1`,
		"upgrade missing previous release": `"reason":"higher-score","score":90,"releaseMatch":1,"previousSource":"subdl","previousScore":70`,
		"upgrade negative previous":        `"reason":"higher-score","score":90,"releaseMatch":1,"previousSource":"subdl","previousScore":-1,"previousReleaseMatch":1`,
		"manual score conflict":            `"reason":"manual","score":90,"releaseMatch":1`,
		"exact hash conflict":              `"reason":"exact-hash","score":90,"releaseMatch":1`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "subtitle_acquisitions.json")
			action := "added"
			if strings.HasPrefix(name, "upgrade") || name == "exact hash conflict" {
				action = "updated"
			}
			content := fmt.Sprintf(`{"version":%d,"records":{},"history":[{"key":"0123456789abcdef:en","action":"%s","source":"subdl","installedAt":%d,%s}]}`, subtitleLedgerVersion, action, time.Now().Unix(), evidence)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			ledger := newSubtitleLedger(dir)
			if ledger.err == nil || ledger.takeSubDLDownload(time.Now()) {
				t.Fatal("invalid history allowed downloads")
			}
			ledger.clearSearches()
			after, err := os.ReadFile(path)
			if err != nil || string(after) != content {
				t.Fatalf("rejected evidence changed persisted state: %q, %v", after, err)
			}
		})
	}
}
