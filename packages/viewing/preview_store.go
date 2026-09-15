package viewing

import (
	"context"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// Acquire reserves one bounded preview worker after expired entries are removed.
func (store PreviewStore) Acquire(ctx context.Context, now time.Time) (func(), error) {
	store.Mutex.Lock()
	store.ExpireLocked(now)
	full := len(*store.Values) >= store.Limit
	if *store.Slots == nil {
		*store.Slots = make(chan struct{}, store.Workers)
	}
	slots := *store.Slots
	store.Mutex.Unlock()
	if full {
		return nil, errors.New("too many viewing import previews are active")
	}
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, errors.New("viewing activity preview was canceled")
	default:
		return nil, errors.New("viewing activity preview capacity is in use")
	}
}

// Add stores one preview and schedules its expiry.
func (store PreviewStore) Add(preview Preview, now time.Time) error {
	store.Mutex.Lock()
	defer store.Mutex.Unlock()
	store.ExpireLocked(now)
	if len(*store.Values) >= store.Limit {
		return errors.New("too many viewing import previews are active")
	}
	preview.Timer = time.AfterFunc(time.Until(preview.ExpiresAt), func() { store.ExpireID(preview.ID, preview.ExpiresAt, time.Now()) })
	(*store.Values)[preview.ID] = preview
	return nil
}

// Get returns a current preview after removing expired entries.
func (store PreviewStore) Get(id string, now time.Time) (Preview, bool) {
	store.Mutex.Lock()
	defer store.Mutex.Unlock()
	store.ExpireLocked(now)
	preview, found := (*store.Values)[id]
	return preview, found
}

// Delete removes a preview and stops its timer.
func (store PreviewStore) Delete(id string) {
	store.Mutex.Lock()
	store.DeleteLocked(id)
	store.Mutex.Unlock()
}

// ExpireID removes the same expired generation of a preview.
func (store PreviewStore) ExpireID(id string, expires, now time.Time) {
	store.Mutex.Lock()
	if preview, found := (*store.Values)[id]; found && preview.ExpiresAt.Equal(expires) && !preview.ExpiresAt.After(now) {
		store.DeleteLocked(id)
	}
	store.Mutex.Unlock()
}

// ExpireLocked removes all previews due at now. The caller must hold Mutex.
func (store PreviewStore) ExpireLocked(now time.Time) {
	for id, preview := range *store.Values {
		if !preview.ExpiresAt.After(now) {
			store.DeleteLocked(id)
		}
	}
}

// DeleteLocked removes a preview while the caller holds Mutex.
func (store PreviewStore) DeleteLocked(id string) {
	if preview, found := (*store.Values)[id]; found && preview.Timer != nil {
		preview.Timer.Stop()
	}
	delete(*store.Values, id)
}

// Prepare runs one bounded source fetch, Library snapshot, plan, and expiry registration.
func Prepare(ctx context.Context, store PreviewStore, fetch func(context.Context) ([]Activity, error), snapshot func() ([]library.Item, error), build func([]Activity, []library.Item) Preview) (Preview, error) {
	release, err := store.Acquire(ctx, time.Now())
	if err != nil {
		return Preview{}, err
	}
	defer release()
	activities, err := fetch(ctx)
	if err != nil {
		return Preview{}, err
	}
	items, err := snapshot()
	if err != nil {
		return Preview{}, errors.New("Kinosail Library is unavailable") //nolint:staticcheck // Kinosail is a proper product name.
	}
	preview := build(activities, items)
	if err := store.Add(preview, time.Now()); err != nil {
		return Preview{}, err
	}
	return preview, nil
}

// ApplyPreview reads, commits, summarizes, and consumes one preview on success.
func ApplyPreview(store PreviewStore, id string, commit func(Preview) (int, int, int, error)) (Summary, error) {
	preview, found := store.Get(id, time.Now())
	if !found {
		return Summary{}, errors.New("import preview expired or was not found")
	}
	applied, conflicts, listsApplied, err := commit(preview)
	result := preview.Summary
	result.Applied, result.Conflicts, result.ListsApplied = applied, result.Conflicts+conflicts, listsApplied
	if err == nil {
		store.Delete(id)
	}
	return result, err
}
