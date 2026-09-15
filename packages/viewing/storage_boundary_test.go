package viewing

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestSyncStorageNumericBoundaries(t *testing.T) { //nolint:cyclop // Exact adjacent values lock every durable counter boundary.
	if !withinMaximum(512, 512) || withinMaximum(513, 512) {
		t.Fatal("maximum boundary changed")
	}
	if total, valid := addWithinMaximum(900, 100, 1000); total != 1000 || !valid {
		t.Fatalf("boundary sum = %d, %t", total, valid)
	}
	if total, valid := addWithinMaximum(901, 100, 1000); total != 901 || valid {
		t.Fatalf("overflowing sum = %d, %t", total, valid)
	}
	if !validSyncCount(0) || !validSyncCount(maximumSyncItems) || validSyncCount(-1) || validSyncCount(maximumSyncItems+1) {
		t.Fatal("summary count boundary changed")
	}
	if !validSyncPosition(0) || !validSyncPosition(maximumSyncItems-1) || validSyncPosition(-1) || validSyncPosition(maximumSyncItems) {
		t.Fatal("playlist position boundary changed")
	}
}

func TestSyncStorageStateBoundaries(t *testing.T) {
	for _, state := range []catalog.PlaybackState{
		{Seconds: 0},
		{Seconds: maximumSyncSeconds, Session: strings.Repeat("s", 512)},
	} {
		if !validPendingState(state) {
			t.Fatalf("valid state rejected: %#v", state)
		}
	}
	for _, state := range []catalog.PlaybackState{
		{Seconds: -1},
		{Seconds: maximumSyncSeconds + 1},
		{Seconds: math.NaN()},
		{Seconds: math.Inf(1)},
		{Session: strings.Repeat("s", 513)},
	} {
		if validPendingState(state) {
			t.Fatalf("invalid state accepted: %#v", state)
		}
	}
	valid := Observation{TargetID: strings.Repeat("t", 512), Signature: strings.Repeat("s", 2048)}
	if !validObservation(strings.Repeat("k", 2048), valid) {
		t.Fatal("observation boundary rejected")
	}
	for name, test := range map[string]struct {
		key         string
		observation Observation
	}{
		"empty key":      {observation: valid},
		"long key":       {key: strings.Repeat("k", 2049), observation: valid},
		"empty target":   {key: "key", observation: Observation{}},
		"long target":    {key: "key", observation: Observation{TargetID: strings.Repeat("t", 513)}},
		"long signature": {key: "key", observation: Observation{TargetID: "target", Signature: strings.Repeat("s", 2049)}},
	} {
		t.Run(name, func(t *testing.T) {
			if validObservation(test.key, test.observation) {
				t.Fatal("invalid observation accepted")
			}
		})
	}
}

func TestNormalizeSyncRejectsEveryPersistedInvariant(t *testing.T) { //nolint:cyclop // Every persisted field is an independent trust boundary.
	const id = "ABCDEFGHIJKLMNOP234567"
	normalize := func(input Input) (Input, error) {
		input.Source = strings.ToLower(input.Source)
		return input, nil
	}
	valid := Sync{ID: id, Source: "PLEX", Interval: "15m", LastError: strings.Repeat("e", 1024), Seen: map[string]Observation{"key": {TargetID: "target"}}}
	normalized, err := normalizeSync(id, valid, normalize)
	if err != nil || normalized.Source != "plex" {
		t.Fatalf("normalized sync = %#v, %v", normalized, err)
	}
	cases := map[string]func(*Sync) (string, func(Input) (Input, error)){
		"normalizer": func(sync *Sync) (string, func(Input) (Input, error)) {
			return id, func(Input) (Input, error) { return Input{}, errors.New("invalid") }
		},
		"map id":     func(sync *Sync) (string, func(Input) (Input, error)) { return "BCDEFGHIJKLMNOP234567A", normalize },
		"invalid id": func(sync *Sync) (string, func(Input) (Input, error)) { sync.ID = "bad"; return "bad", normalize },
		"interval":   func(sync *Sync) (string, func(Input) (Input, error)) { sync.Interval = "weekly"; return id, normalize },
		"last error": func(sync *Sync) (string, func(Input) (Input, error)) { sync.LastError += "e"; return id, normalize },
		"summary": func(sync *Sync) (string, func(Input) (Input, error)) {
			sync.LastResult.Applied = -1
			return id, normalize
		},
		"pending": func(sync *Sync) (string, func(Input) (Input, error)) {
			sync.Pending = &SyncPending{Changes: []ProgressChange{{}}}
			return id, normalize
		},
		"observation": func(sync *Sync) (string, func(Input) (Input, error)) {
			sync.Seen = map[string]Observation{"": {TargetID: "target"}}
			return id, normalize
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			sync := valid
			key, callback := change(&sync)
			if restored, restoreErr := normalizeSync(key, sync, callback); restoreErr == nil || restored.ID != "" {
				t.Fatalf("invalid sync restored: %#v, %v", restored, restoreErr)
			}
		})
	}
}

func TestSyncStorageExactLimits(t *testing.T) { //nolint:cyclop // Exact maxima distinguish accepted durable state from oversize state.
	for _, interval := range []string{"15m", "1h", "6h", "24h"} {
		if !ValidSyncInterval(interval) {
			t.Fatalf("valid interval rejected: %q", interval)
		}
	}
	if ValidSyncInterval("") {
		t.Fatal("empty interval accepted")
	}
	if !ValidSummary(Summary{Applied: maximumSyncItems}) || ValidSummary(Summary{Applied: maximumSyncItems + 1}) {
		t.Fatal("summary maximum changed")
	}
	exact := strings.Repeat("e", 1024)
	if SafeError(errors.New(exact)) != exact || len(SafeError(errors.New(exact+"e"))) != 1024 {
		t.Fatal("safe error boundary changed")
	}

	document := []byte(`{"ABCDEFGHIJKLMNOP234567":{"id":"ABCDEFGHIJKLMNOP234567","interval":"15m","lastResult":{}}}`)
	document = append(document, bytes.Repeat([]byte{' '}, MaximumSyncStateSize-len(document))...)
	file := filepath.Join(t.TempDir(), "viewing_imports.json")
	if err := os.WriteFile(file, document, 0o600); err != nil {
		t.Fatal(err)
	}
	syncs, err := LoadSyncs(file, func(input Input) (Input, error) { return input, nil })
	if err != nil || len(syncs) != 1 {
		t.Fatalf("maximum-size state = %#v, %v", syncs, err)
	}
}
