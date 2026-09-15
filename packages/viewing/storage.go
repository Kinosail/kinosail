package viewing

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
)

// MaximumSyncStateSize bounds the complete durable sync document.
const MaximumSyncStateSize = 1 << 24

const (
	maximumSyncReadSize  = 16_777_217
	maximumSyncItems     = 100_000
	maximumSyncPlaylists = 1_000
	maximumSyncSeconds   = 31_622_400
)

// LoadSyncs strictly restores a complete sync document before returning any state.
func LoadSyncs(file string, normalize func(Input) (Input, error)) (map[string]Sync, error) { //nolint:cyclop // Restore validates every persisted invariant before accepting any state.
	reader, err := os.Open(file) //nolint:gosec // The app owns the configured durable-state path.
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Sync), nil
		}
		return nil, errors.New("viewing activity sync state could not be opened")
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maximumSyncReadSize))
	if err != nil {
		return nil, errors.New("viewing activity sync state could not be read")
	}
	if len(data) > MaximumSyncStateSize {
		return nil, errors.New("viewing activity sync state could not be read")
	}
	decoder, syncs := json.NewDecoder(bytes.NewReader(data)), make(map[string]Sync)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&syncs) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("viewing activity sync state is invalid")
	}
	for id, sync := range syncs {
		normalized, normalizeErr := normalizeSync(id, sync, normalize)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		syncs[id] = normalized
	}
	return syncs, nil
}

func normalizeSync(id string, sync Sync, normalize func(Input) (Input, error)) (Sync, error) { //nolint:cyclop // Durable state validates every invariant before admission.
	input, inputErr := normalize(Input{Source: sync.Source, URL: sync.URL, Token: sync.Token, SourceUser: sync.SourceUser, ProfileID: sync.ProfileID})
	if inputErr != nil || id != sync.ID || !ValidID(id) || !ValidSyncInterval(sync.Interval) {
		return Sync{}, errors.New("viewing activity sync state is invalid")
	}
	if !withinMaximum(len(sync.Seen), maximumSyncItems) || !withinMaximum(len(sync.LastError), 1024) || !ValidSummary(sync.LastResult) || !ValidPending(sync.Pending) {
		return Sync{}, errors.New("viewing activity sync state is invalid")
	}
	for key, observation := range sync.Seen {
		if !validObservation(key, observation) {
			return Sync{}, errors.New("viewing activity sync state is invalid")
		}
	}
	sync.Source, sync.URL, sync.Token, sync.SourceUser, sync.ProfileID = input.Source, input.URL, input.Token, input.SourceUser, input.ProfileID
	return sync, nil
}

func validObservation(key string, observation Observation) bool {
	if key == "" || !withinMaximum(len(key), 2048) || observation.TargetID == "" {
		return false
	}
	return withinMaximum(len(observation.TargetID), 512) && withinMaximum(len(observation.Signature), 2048)
}

// ValidPending reports whether a durable recovery batch satisfies every bound.
func ValidPending(pending *SyncPending) bool { //nolint:cyclop,gocognit // Persisted recovery state is a trust boundary.
	if pending == nil {
		return true
	}
	if !withinMaximum(len(pending.Changes), maximumSyncItems) {
		return false
	}
	if !withinMaximum(len(pending.ListChanges), maximumSyncItems) {
		return false
	}
	if !ValidSummary(pending.Summary) {
		return false
	}
	return validPendingProgress(pending.Changes) && validPendingLists(pending.ListChanges)
}

func validPendingProgress(changes []ProgressChange) bool {
	for _, change := range changes {
		if change.TargetID == "" || !withinMaximum(len(change.TargetID), 512) || !validPendingState(change.State) || !validPendingState(change.Expected) {
			return false
		}
	}
	return true
}

func validPendingState(state catalog.PlaybackState) bool {
	return state.Seconds >= 0 && state.Seconds <= maximumSyncSeconds && withinMaximum(len(state.Session), 512)
}

func validPendingLists(changes []ListChange) bool {
	memberships := 0
	for _, change := range changes {
		if change.TargetID == "" || !withinMaximum(len(change.TargetID), 512) || !withinMaximum(len(change.Playlists), maximumSyncPlaylists) {
			return false
		}
		var valid bool
		memberships, valid = addWithinMaximum(memberships, len(change.Playlists), maximumSyncItems)
		if !valid {
			return false
		}
		for name, position := range change.Playlists {
			if !catalog.ValidListName(name) || !validSyncPosition(position) {
				return false
			}
		}
	}
	return true
}

// ValidSummary reports whether all persisted counters are finite bounded counts.
func ValidSummary(summary Summary) bool {
	values := []int{summary.SourceItems, summary.Activity, summary.Matched, summary.Importable, summary.Unchanged, summary.Ambiguous, summary.Unmatched, summary.Conflicts, summary.Favorites, summary.PlaylistItems, summary.Applied, summary.ListsApplied}
	for _, value := range values {
		if !validSyncCount(value) {
			return false
		}
	}
	return true
}

// ValidSyncInterval reports whether a recurring import interval is supported.
func ValidSyncInterval(interval string) bool {
	return interval == "15m" || interval == "1h" || interval == "6h" || interval == "24h"
}

// CloneSyncs returns a deep copy of recurring import state.
func CloneSyncs(source map[string]Sync) map[string]Sync {
	result := make(map[string]Sync, len(source))
	for id, sync := range source {
		sync.Seen = CloneObservations(sync.Seen)
		if sync.Pending != nil {
			pending := *sync.Pending
			pending.Changes = slices.Clone(pending.Changes)
			pending.ListChanges = slices.Clone(pending.ListChanges)
			for index := range pending.ListChanges {
				pending.ListChanges[index].Playlists = maps.Clone(pending.ListChanges[index].Playlists)
			}
			sync.Pending = &pending
		}
		result[id] = sync
	}
	return result
}

// CloneObservations returns a detached source-observation map.
func CloneObservations(source map[string]Observation) map[string]Observation {
	return maps.Clone(source)
}

// SafeError flattens and bounds an error before durable or visible storage.
func SafeError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.ReplaceAll(err.Error(), "\n", " ")
	return value[:min(len(value), 1024)]
}

func withinMaximum(value, maximum int) bool { return value <= maximum }

func addWithinMaximum(current, added, maximum int) (int, bool) {
	if added > maximum-current {
		return current, false
	}
	return current + added, true
}

func validSyncCount(value int) bool { return value >= 0 && value <= maximumSyncItems }

func validSyncPosition(value int) bool { return value >= 0 && value < maximumSyncItems }
