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

type ProgressViewer struct {
	ID    string
	Owner bool
}

type ProgressAuditor interface {
	ResolveTitle(string) string
	Playback(*http.Request, string, string, string, float64, bool)
}

type RequestProgressStore struct {
	mu          sync.RWMutex
	persistMu   sync.Mutex
	coalescing  bool
	flushFailed bool
	dirty       map[string]struct{}
	file        string
	values      map[string]PlaybackState
	persist     func(string, any) error
	database    *documentdb.Store
	entries     bool
	err         error
	viewer      func(*http.Request) ProgressViewer
	project     func(*http.Request, library.Item, PlaybackState) any
	auditor     ProgressAuditor
}

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
		store.err = database.EnableEntries(progressDocument)
		store.entries = store.err == nil
	}
	if store.values == nil {
		store.values = make(map[string]PlaybackState)
	}
	return store
}

func (store *RequestProgressStore) Err() error { return store.err }

func (store *RequestProgressStore) Database() *documentdb.Store { return store.database }

func (store *RequestProgressStore) SetAuditor(auditor ProgressAuditor) { store.auditor = auditor }

func (store *RequestProgressStore) SetPersistence(persist func(string, any) error) {
	store.persist = persist
	store.entries = false
}

func (store *RequestProgressStore) Replace(values map[string]PlaybackState) {
	store.persistMu.Lock()
	defer store.persistMu.Unlock()
	store.mu.Lock()
	defer store.mu.Unlock()
	if values == nil {
		values = make(map[string]PlaybackState)
	}
	store.values = values
	clear(store.dirty)
}

func (store *RequestProgressStore) Len() int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.values)
}

func (store *RequestProgressStore) Storage() ProgressStorage {
	storage := ProgressStorage{Mutex: &store.mu, PersistMutex: &store.persistMu, Values: &store.values, File: store.file, Persist: store.persist}
	if store.entries {
		storage.PersistEntry = func(key string, value PlaybackState) error {
			return store.database.SaveEntry(progressDocument, key, value)
		}
	}
	return storage
}

func (store *RequestProgressStore) BrowseAccess(listMutex *sync.RWMutex, listed *map[string]bool, request *http.Request, visible func(library.Item) bool) BrowseAccess {
	viewer := store.viewer(request)
	return NewBrowseAccess(store.mu.RLocker(), listMutex, &store.values, listed, viewer.ID, viewer.Owner, visible)
}

func (store *RequestProgressStore) Get(request *http.Request, id string) PlaybackState {
	viewer := store.viewer(request)
	return store.GetFor(viewer.ID, viewer.Owner, id)
}

func (store *RequestProgressStore) GetFor(profile string, owner bool, id string) PlaybackState {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return ViewerProgress(store.values, profile, owner, id)
}

func (store *RequestProgressStore) ClientItem(request *http.Request, item library.Item) any {
	return store.project(request, item, store.Get(request, item.ID))
}

func (store *RequestProgressStore) ClientItems(request *http.Request, items []library.Item) any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, store.ClientItem(request, item))
	}
	return result
}

func (store *RequestProgressStore) Watched(request *http.Request, id string) bool {
	return store.Get(request, id).Watched
}

func (store *RequestProgressStore) ReaderPage(request *http.Request, id string, total int) int {
	page := store.Get(request, id).ReaderPage
	if page < 1 || page > total {
		return 1
	}
	return page
}

func (store *RequestProgressStore) SetReaderPage(request *http.Request, id string, page, total int) error {
	return store.SetReaderPosition(request, id, page, total, 0)
}

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

func (store *RequestProgressStore) History(request *http.Request, items []library.Item, page Page) ([]library.Item, int) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	viewer := store.viewer(request)
	return ProgressHistoryPage(store.values, viewer.ID, viewer.Owner, items, page)
}

func (store *RequestProgressStore) Dismiss(request *http.Request, id string) error {
	key := store.viewer(request).ID + ":" + id
	_, _, _, err := store.Update(key, func(state PlaybackState) (PlaybackState, bool, error) {
		state.Dismissed = true
		return state, true, nil
	})
	return err
}

func (store *RequestProgressStore) RecentAdmin(items []library.Item, names map[string]string) []PlaybackView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return RecentAdminProgress(store.values, items, names)
}

func (store *RequestProgressStore) Set(request *http.Request, id string, seconds float64, watched *bool) error {
	_, err := store.SetRevision(request, id, seconds, watched, "", 0)
	return err
}

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

func (store *RequestProgressStore) Stop(request *http.Request, id string, seconds float64) {
	_ = store.Flush()
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
