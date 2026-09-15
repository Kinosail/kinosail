package catalog

import (
	"errors"
	"math"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

// ProgressViewer identifies the Viewer whose private playback state is being accessed.
type ProgressViewer struct {
	ID    string
	Owner bool
}

// ProgressAuditor records the lifecycle events derived from accepted progress updates.
type ProgressAuditor interface {
	ResolveTitle(string) string
	Playback(*http.Request, string, string, string, float64, bool)
}

// RequestProgressStore owns durable playback state for HTTP-facing applications.
type RequestProgressStore struct {
	mu, persistMu sync.Mutex
	file          string
	values        map[string]PlaybackState
	persist       func(string, any) error
	database      *documentdb.Store
	entries       bool
	err           error
	viewer        func(*http.Request) ProgressViewer
	project       func(*http.Request, library.Item, PlaybackState) any
	auditor       ProgressAuditor
}

// NewRequestProgressStore restores an application's progress state through its persistence boundary.
func NewRequestProgressStore(dataDir string, database *documentdb.Store, load func(string, any) (bool, error), persist func(string, any) error, viewer func(*http.Request) ProgressViewer, project func(*http.Request, library.Item, PlaybackState) any) *RequestProgressStore {
	store := &RequestProgressStore{values: make(map[string]PlaybackState), persist: persist, database: database, viewer: viewer, project: project}
	if dataDir == "" {
		return store
	}
	store.file = filepath.Join(dataDir, "progress.json")
	if _, err := load(store.file, &store.values); err != nil {
		store.err = err
	} else {
		store.err = ValidateStoredProgress(store.values)
	}
	if store.err == nil && database != nil {
		store.err = database.EnableEntries("progress.json")
		store.entries = store.err == nil
	}
	if store.values == nil {
		store.values = make(map[string]PlaybackState)
	}
	return store
}

// Err returns the progress restoration error, if any.
func (store *RequestProgressStore) Err() error { return store.err }

// Database returns the database backing this store.
func (store *RequestProgressStore) Database() *documentdb.Store { return store.database }

// SetAuditor connects playback lifecycle events after authentication is initialized.
func (store *RequestProgressStore) SetAuditor(auditor ProgressAuditor) { store.auditor = auditor }

// SetPersistence replaces the persistence boundary.
func (store *RequestProgressStore) SetPersistence(persist func(string, any) error) {
	store.persist = persist
	store.entries = false
}

// Replace publishes a complete progress snapshot.
func (store *RequestProgressStore) Replace(values map[string]PlaybackState) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if values == nil {
		values = make(map[string]PlaybackState)
	}
	store.values = values
}

// Len returns the number of stored progress records.
func (store *RequestProgressStore) Len() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.values)
}

// Storage exposes the validated transaction boundary for batch operations.
func (store *RequestProgressStore) Storage() ProgressStorage {
	storage := ProgressStorage{Mutex: &store.mu, PersistMutex: &store.persistMu, Values: &store.values, File: store.file, Persist: store.persist}
	if store.entries {
		storage.PersistEntry = func(key string, value PlaybackState) error {
			return store.database.SaveEntry("progress.json", key, value)
		}
	}
	return storage
}

// Update commits one validated state change.
func (store *RequestProgressStore) Update(key string, change ProgressChange) (PlaybackState, PlaybackState, bool, error) { //nolint:contextcheck // A validated state commit must finish after client cancellation.
	return store.Storage().Update(key, change)
}

// BrowseAccess projects this store into the catalog browsing transaction.
func (store *RequestProgressStore) BrowseAccess(listMutex *sync.RWMutex, listed *map[string]bool, request *http.Request, visible func(library.Item) bool) BrowseAccess {
	viewer := store.viewer(request)
	return NewBrowseAccess(&store.mu, listMutex, &store.values, listed, viewer.ID, viewer.Owner, visible)
}

// Get returns one item state for the current Viewer.
func (store *RequestProgressStore) Get(request *http.Request, id string) PlaybackState {
	viewer := store.viewer(request)
	return store.GetFor(viewer.ID, viewer.Owner, id)
}

// GetFor returns one item state for an explicit Viewer.
func (store *RequestProgressStore) GetFor(profile string, owner bool, id string) PlaybackState {
	store.mu.Lock()
	defer store.mu.Unlock()
	return ViewerProgress(store.values, profile, owner, id)
}

// ClientItem projects one item for the current Viewer.
func (store *RequestProgressStore) ClientItem(request *http.Request, item library.Item) any {
	return store.project(request, item, store.Get(request, item.ID))
}

// ClientItems projects items for the current Viewer.
func (store *RequestProgressStore) ClientItems(request *http.Request, items []library.Item) any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, store.ClientItem(request, item))
	}
	return result
}

// Watched reports whether the current Viewer watched an item.
func (store *RequestProgressStore) Watched(request *http.Request, id string) bool {
	return store.Get(request, id).Watched
}

// ReaderPage returns a valid one-based page for an item.
func (store *RequestProgressStore) ReaderPage(request *http.Request, id string, total int) int {
	page := store.Get(request, id).ReaderPage
	if page < 1 || page > total {
		return 1
	}
	return page
}

// SetReaderPage validates and stores one reader position.
func (store *RequestProgressStore) SetReaderPage(request *http.Request, id string, page, total int) error {
	return store.SetReaderPosition(request, id, page, total, 0)
}

// SetReaderPosition saves a chapter and its normalized reading offset atomically.
func (store *RequestProgressStore) SetReaderPosition(request *http.Request, id string, page, total int, offset float64) error {
	if total < 1 || total > 10000 || page < 1 || page > total || math.IsNaN(offset) || math.IsInf(offset, 0) || offset < 0 || offset > 1 {
		return errors.New("reader page is out of range")
	}
	key := store.viewer(request).ID + ":" + id
	_, _, _, err := store.Update(key, func(state PlaybackState) (PlaybackState, bool, error) {
		state.ReaderPage, state.ReaderOffset, state.Updated, state.Dismissed = page, offset, time.Now(), false
		return state, true, nil
	})
	return err
}

// Active returns resumable items for the current Viewer.
func (store *RequestProgressStore) Active(request *http.Request, items []library.Item) []ResumeItem {
	store.mu.Lock()
	defer store.mu.Unlock()
	viewer := store.viewer(request)
	return ActiveProgress(store.values, viewer.ID, viewer.Owner, items)
}

// Recent returns up to twenty most recently active items.
func (store *RequestProgressStore) Recent(request *http.Request, items []library.Item) []library.Item {
	items = store.History(request, items)
	return items[:min(20, len(items))]
}

// History returns items ordered by the current Viewer's latest activity.
func (store *RequestProgressStore) History(request *http.Request, items []library.Item) []library.Item {
	store.mu.Lock()
	defer store.mu.Unlock()
	viewer := store.viewer(request)
	return ProgressHistory(store.values, viewer.ID, viewer.Owner, items)
}

// Dismiss removes an item from continue watching for the current Viewer.
func (store *RequestProgressStore) Dismiss(request *http.Request, id string) error {
	key := store.viewer(request).ID + ":" + id
	_, _, _, err := store.Update(key, func(state PlaybackState) (PlaybackState, bool, error) {
		state.Dismissed = true
		return state, true, nil
	})
	return err
}

// RecentAdmin returns recent playback activity across Viewer profiles.
func (store *RequestProgressStore) RecentAdmin(items []library.Item, names map[string]string) []PlaybackView {
	store.mu.Lock()
	defer store.mu.Unlock()
	return RecentAdminProgress(store.values, items, names)
}

// Set stores progress without optimistic session ordering.
func (store *RequestProgressStore) Set(request *http.Request, id string, seconds float64, watched *bool) error {
	_, err := store.SetRevision(request, id, seconds, watched, "", 0)
	return err
}

// SetRevision stores progress with optimistic session ordering and emits accepted lifecycle events.
func (store *RequestProgressStore) SetRevision(request *http.Request, id string, seconds float64, watched *bool, session string, revision uint64) (bool, error) {
	key := store.viewer(request).ID + ":" + id
	previous, state, accepted, err := store.Update(key, ProgressRevision(seconds, watched, session, revision, time.Now()))
	started, completed := ProgressAudit(previous, state, seconds, watched, accepted, err)
	if started {
		store.audit(request, "playback.started", id, seconds, false)
	}
	if completed {
		store.audit(request, "playback.completed", id, seconds, true)
	}
	return accepted, err
}

// Stop records a playback stop without changing durable progress.
func (store *RequestProgressStore) Stop(request *http.Request, id string, seconds float64) {
	store.audit(request, "playback.stopped", id, seconds, false)
}

func (store *RequestProgressStore) audit(request *http.Request, action, id string, seconds float64, watched bool) {
	if store.auditor == nil {
		return
	}
	title := id
	if resolved := store.auditor.ResolveTitle(id); resolved != "" {
		title = resolved
	}
	store.auditor.Playback(request, action, id, title, seconds, watched)
}
