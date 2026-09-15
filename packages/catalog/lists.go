package catalog

import (
	"context"
	"errors"
	"maps"
	"strings"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

// PlaylistRule defines one saved smart-list query.
type PlaylistRule struct {
	Kind  string `json:"kind,omitempty"`
	Query string `json:"query,omitempty"`
	Sort  string `json:"sort,omitempty"`
}

// ValidListName reports whether a playlist or Collection name is bounded and path-safe.
func ValidListName(name string) bool {
	return name != "" && len(name) <= 64 && !strings.Contains(name, "/")
}

// ListAdditions counts import changes that are not already present.
func ListAdditions(state ListState, viewer, id string, favorite bool, playlists map[string]int) int {
	count := 0
	if favorite && !state.Values[viewer+":"+id] {
		count++
	}
	for name := range playlists {
		key := viewer + ":" + name
		if ValidListName(name) && state.Smart[key].Sort == "" && !state.Playlists[key][id] {
			count++
		}
	}
	return count
}

// ListState is the complete durable Library curation state.
type ListState struct {
	Values        map[string]bool
	Playlists     map[string]map[string]bool
	PlaylistOrder map[string][]string
	Smart         map[string]PlaylistRule
}

// ListPaths names the legacy JSON documents for curation state.
type ListPaths struct {
	Values, Playlists, PlaylistOrder, Smart string
}

// DocumentMask selects durable curation documents for one atomic commit.
type DocumentMask uint8

const (
	// ListedDocument stores My List membership.
	ListedDocument DocumentMask = 1 << iota
	// PlaylistsDocument stores playlist and Collection membership.
	PlaylistsDocument
	// PlaylistOrderDocument stores stable playlist ordering.
	PlaylistOrderDocument
	// SmartListDocument stores smart playlist rules.
	SmartListDocument
)

// ListStorage binds shared curation operations to app-owned fields.
// Callers must hold their state lock across each operation.
type ListStorage struct {
	Values        *map[string]bool
	Playlists     *map[string]map[string]bool
	PlaylistOrder *map[string][]string
	Smart         *map[string]PlaylistRule
	Paths         ListPaths
	Database      *documentdb.Store
	Persist       func(string, any) error
	Load          func(string, any) (bool, error)
	LoadError     *error
}

// Restore loads and publishes all valid curation state at once.
func (storage ListStorage) Restore() error {
	state, err := LoadListState(storage.Paths, storage.Load)
	if err == nil {
		storage.Assign(state)
	}
	return err
}

// State returns one detached initialized curation snapshot.
func (storage ListStorage) State() ListState {
	return CloneListState(*storage.Values, *storage.Playlists, *storage.PlaylistOrder, *storage.Smart)
}

// Commit persists and then publishes the selected curation documents.
func (storage ListStorage) Commit(ctx context.Context, state ListState, documents DocumentMask) error {
	if *storage.LoadError != nil {
		return *storage.LoadError
	}
	if err := CommitListState(ctx, storage.Database, storage.Paths, storage.Persist, state, uint8(documents)); err != nil {
		return err
	}
	storage.Assign(state)
	return nil
}

// Assign publishes a complete detached curation state.
func (storage ListStorage) Assign(state ListState) {
	*storage.Values, *storage.Playlists, *storage.PlaylistOrder, *storage.Smart = state.Values, state.Playlists, state.PlaylistOrder, state.Smart
}

// LoadListState reads and validates all canonical curation documents.
func LoadListState(paths ListPaths, load func(string, any) (bool, error)) (ListState, error) {
	state := ListState{}
	for _, document := range []struct {
		path   string
		target any
	}{
		{paths.Values, &state.Values},
		{paths.Playlists, &state.Playlists},
		{paths.PlaylistOrder, &state.PlaylistOrder},
		{paths.Smart, &state.Smart},
	} {
		if _, err := load(document.path, document.target); err != nil {
			return ListState{}, err
		}
	}
	state = CloneListState(state.Values, state.Playlists, state.PlaylistOrder, state.Smart)
	if err := ValidateListState(state); err != nil {
		return ListState{}, err
	}
	return state, nil
}

// CloneListState returns a detached, initialized state snapshot.
func CloneListState(values map[string]bool, playlists map[string]map[string]bool, order map[string][]string, smart map[string]PlaylistRule) ListState {
	state := ListState{maps.Clone(values), make(map[string]map[string]bool, len(playlists)), make(map[string][]string, len(order)), maps.Clone(smart)}
	for key, selected := range playlists {
		state.Playlists[key] = maps.Clone(selected)
	}
	for key, selected := range order {
		state.PlaylistOrder[key] = append([]string(nil), selected...)
	}
	if state.Values == nil {
		state.Values = make(map[string]bool)
	}
	if state.Smart == nil {
		state.Smart = make(map[string]PlaylistRule)
	}
	return state
}

// ValidateListState rejects unbounded or malformed durable curation state.
func ValidateListState(state ListState) error {
	if !validListCardinality(state) || !validListValues(state.Values) || !validPlaylists(state.Playlists) || !validPlaylistOrder(state.PlaylistOrder) || !validSmartLists(state.Smart) {
		return errors.New("persisted list state is invalid")
	}
	return nil
}

func validListCardinality(state ListState) bool {
	return len(state.Values) <= 1_000_000 && len(state.Playlists) <= 10_000 && len(state.PlaylistOrder) <= 10_000 && len(state.Smart) <= 10_000
}

func validListValues(values map[string]bool) bool {
	for key := range values {
		if key == "" || len(key) > 256 {
			return false
		}
	}
	return true
}

func validPlaylists(playlists map[string]map[string]bool) bool {
	for key, selected := range playlists {
		if key == "" || len(key) > 256 || len(selected) > 10_000 {
			return false
		}
	}
	return true
}

func validPlaylistOrder(order map[string][]string) bool {
	for key, selected := range order {
		if key == "" || len(key) > 256 || len(selected) > 10_000 || !allUnique(selected) {
			return false
		}
	}
	return true
}

func validSmartLists(smart map[string]PlaylistRule) bool {
	for key, rule := range smart {
		if key == "" || len(key) > 256 || len(rule.Query) > 200 || !oneOf(rule.Kind, "", "video", "audio", "book", "photo") || !oneOf(rule.Sort, "title", "added", "year") {
			return false
		}
	}
	return true
}

// ListDocuments returns the selected canonical document values.
func ListDocuments(state ListState, mask uint8) map[string]any {
	updates := make(map[string]any, 4)
	if mask&1 != 0 {
		updates["lists.json"] = state.Values
	}
	if mask&2 != 0 {
		updates["playlists.json"] = state.Playlists
	}
	if mask&4 != 0 {
		updates["playlist_order.json"] = state.PlaylistOrder
	}
	if mask&8 != 0 {
		updates["smart_playlists.json"] = state.Smart
	}
	return updates
}

// CommitListState persists all selected documents as one database transaction or ordered file writes.
func CommitListState(ctx context.Context, database *documentdb.Store, paths ListPaths, persist func(string, any) error, state ListState, mask uint8) error {
	if database != nil {
		return database.SaveJSONBatchContext(ctx, ListDocuments(state, mask))
	}
	for _, document := range []struct {
		selected bool
		path     string
		value    any
	}{
		{mask&1 != 0, paths.Values, state.Values},
		{mask&2 != 0, paths.Playlists, state.Playlists},
		{mask&4 != 0, paths.PlaylistOrder, state.PlaylistOrder},
		{mask&8 != 0, paths.Smart, state.Smart},
	} {
		if document.selected && document.path != "" {
			if err := persist(document.path, document.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func allUnique(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
