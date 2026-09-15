package viewing

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *Manager) CreateSync(previewID, interval string) (SyncView, error) { //nolint:cyclop,funlen // Persistence and rollback keep sync creation atomic with its initial import.
	duration, ok := syncIntervals[interval]
	if !ok {
		return SyncView{}, errors.New("sync interval must be 15m, 1h, 6h, or 24h")
	}
	manager.mu.Lock()
	manager.expirePreviews(time.Now())
	preview, found := manager.previews[previewID]
	if !found {
		manager.mu.Unlock()
		return SyncView{}, errors.New("import preview expired or was not found")
	}
	profile, found := manager.config.Profile(preview.ProfileID)
	if !found {
		manager.mu.Unlock()
		return SyncView{}, errors.New("destination Viewer Profile was not found")
	}
	now := time.Now().UTC()
	sync := Sync{ID: rand.Text(), Source: preview.Input.Source, URL: preview.Input.URL, Token: preview.Input.Token, SourceUser: preview.Input.SourceUser, ProfileID: preview.Input.ProfileID, Interval: interval, NextRun: now, Seen: make(map[string]Observation), Pending: &SyncPending{Summary: preview.Summary, Changes: preview.Changes, ListChanges: preview.ListChanges}}
	items, _ := manager.config.Snapshot()
	for _, activity := range preview.Activities {
		if target, status, _ := Match(activity, items); status == "matched" && (activity.Watched || activity.Seconds > 0) {
			sync.Seen[ActivityKey(activity)] = Observation{TargetID: target.ID, Signature: Signature(activity)}
		}
	}
	sync.LastRun, sync.LastResult = now, preview.Summary
	sync.LastResult.Applied, sync.LastResult.ListsApplied = len(preview.Changes), preview.Summary.Importable-len(preview.Changes)
	previous, syncs := CloneSyncs(manager.syncs), CloneSyncs(manager.syncs)
	syncs[sync.ID] = sync
	if err := manager.save(syncs); err != nil {
		manager.mu.Unlock()
		return SyncView{}, err
	}
	applied, conflicts, listsApplied, err := manager.config.Commit(profile, preview.Changes, preview.ListChanges)
	if err != nil {
		if rollbackErr := manager.save(previous); rollbackErr != nil {
			manager.syncs = syncs
			err = errors.Join(err, errors.New("viewing sync rollback failed"))
		}
		manager.mu.Unlock()
		return SyncView{}, err
	}
	sync.LastRun, sync.LastResult = now, preview.Summary
	sync.LastResult.Applied, sync.LastResult.ListsApplied = applied, listsApplied
	sync.LastResult.Conflicts = preview.Summary.Conflicts + conflicts
	sync.NextRun, sync.Pending = now.Add(duration), nil
	syncs[sync.ID] = sync
	if err := manager.save(syncs); err != nil {
		pending := manager.syncs
		pending[sync.ID] = Sync{ID: sync.ID, Source: sync.Source, URL: sync.URL, Token: sync.Token, SourceUser: sync.SourceUser, ProfileID: sync.ProfileID, Interval: sync.Interval, NextRun: now, Seen: sync.Seen, Pending: &SyncPending{Summary: preview.Summary, Changes: preview.Changes, ListChanges: preview.ListChanges}, LastError: "initial import finalization will retry"}
		manager.deletePreviewLocked(previewID)
		manager.syncs, manager.previews = pending, withoutViewingPreview(manager.previews, previewID)
		view := manager.viewLocked(pending[sync.ID])
		manager.mu.Unlock()
		return view, nil //nolint:nilerr // The durable pending record owns retrying finalization after the import committed.
	}
	manager.deletePreviewLocked(previewID)
	manager.syncs, manager.previews = syncs, withoutViewingPreview(manager.previews, previewID)
	view := manager.viewLocked(sync)
	manager.mu.Unlock()
	return view, nil
}

func withoutViewingPreview(previews map[string]Preview, id string) map[string]Preview {
	result := make(map[string]Preview, len(previews))
	for key, value := range previews {
		if key != id {
			result[key] = value
		}
	}
	return result
}

var syncIntervals = map[string]time.Duration{"15m": 900_000_000_000, "1h": 3_600_000_000_000, "6h": 21_600_000_000_000, "24h": 86_400_000_000_000}

func (manager *Manager) RunSync(ctx context.Context, id string) (SyncView, error) {
	select {
	case manager.syncSlots <- struct{}{}:
		defer func() { <-manager.syncSlots }()
	case <-ctx.Done():
		return SyncView{}, errors.New("viewing activity sync was canceled")
	default:
		return SyncView{}, errors.New("viewing activity sync capacity is in use")
	}
	return manager.runReserved(ctx, id)
}

func (manager *Manager) runReserved(ctx context.Context, id string) (SyncView, error) { //nolint:contextcheck // A fetched sync batch is finalized durably once reconciliation begins.
	manager.mu.Lock()
	sync, found := manager.syncs[id]
	if !found {
		manager.mu.Unlock()
		return SyncView{}, errors.New("viewing activity sync was not found")
	}
	if sync.Running {
		manager.mu.Unlock()
		return SyncView{}, errors.New("viewing activity sync is already running")
	}
	sync.Running, sync.Queued = true, false
	manager.syncs[id] = sync
	manager.mu.Unlock()

	result, seen := Summary{}, sync.Seen
	var runErr error
	if sync.Pending != nil {
		result, runErr = manager.applyPending(sync) //nolint:contextcheck // A fetched batch is already accepted and must finalize durably.
	} else {
		input := Input{Source: sync.Source, URL: sync.URL, Token: sync.Token, SourceUser: sync.SourceUser, ProfileID: sync.ProfileID}
		activities, fetchErr := Fetch(ctx, manager.config.Client, input, false)
		runErr = fetchErr
		if fetchErr == nil {
			result, seen, runErr = manager.pull(sync, activities, time.Now().UTC())
		}
	}

	manager.mu.Lock()
	stored, stillExists := manager.syncs[id]
	if stillExists {
		stored.Running, stored.LastRun, stored.NextRun = false, time.Now().UTC(), time.Now().UTC().Add(syncIntervals[stored.Interval])
		stored.LastResult = result
		if runErr != nil {
			stored.LastError = SafeError(runErr)
		} else {
			stored.LastError, stored.Seen = "", seen
		}
		manager.syncs[id] = stored
		if saveErr := manager.save(manager.syncs); runErr == nil {
			runErr = saveErr
		}
	}
	view := manager.viewLocked(stored)
	manager.mu.Unlock()
	return view, runErr
}

func (manager *Manager) applyPending(sync Sync) (Summary, error) {
	profile, found := manager.config.Profile(sync.ProfileID)
	if !found {
		return Summary{}, errors.New("destination Viewer Profile was not found")
	}
	applied, conflicts, listsApplied, err := manager.config.Commit(profile, sync.Pending.Changes, sync.Pending.ListChanges)
	result := sync.Pending.Summary
	result.Applied, result.Conflicts, result.ListsApplied = applied, result.Conflicts+conflicts, listsApplied
	if err != nil {
		return result, err
	}
	manager.mu.Lock()
	syncs := CloneSyncs(manager.syncs)
	stored := syncs[sync.ID]
	stored.Pending = nil
	syncs[sync.ID] = stored
	if err := manager.save(syncs); err != nil {
		manager.mu.Unlock()
		return result, err
	}
	manager.syncs = syncs
	manager.mu.Unlock()
	return result, nil
}

func (manager *Manager) pull(sync Sync, activities []Activity, now time.Time) (Summary, map[string]Observation, error) {
	profile, found := manager.config.Profile(sync.ProfileID)
	if !found {
		return Summary{}, sync.Seen, errors.New("destination Viewer Profile was not found")
	}
	items, err := manager.config.Snapshot()
	if err != nil {
		return Summary{}, sync.Seen, errors.New("Kinosail Library is unavailable") //nolint:staticcheck // Kinosail is a proper product name.
	}
	result, changes, seen := Summary{SourceItems: len(activities)}, make([]ProgressChange, 0), CloneObservations(sync.Seen)
	for _, activity := range activities {
		manager.pullActivity(profile, sync, items, activity, now, &result, &changes, seen)
	}
	applied, conflicts, err := manager.config.ImportProgress(profile, changes)
	result.Applied, result.Conflicts = applied, result.Conflicts+conflicts
	return result, seen, err
}

func (manager *Manager) pullActivity(profile Profile, sync Sync, items []library.Item, activity Activity, now time.Time, result *Summary, changes *[]ProgressChange, seen map[string]Observation) { //nolint:cyclop // Activity reconciliation remains below the repository complexity ceiling.
	key, signature := ActivityKey(activity), Signature(activity)
	previous, observed := sync.Seen[key]
	if !activity.Watched && activity.Seconds <= 0 && !observed {
		return
	}
	result.Activity++
	if observed && signature == previous.Signature {
		result.Unchanged++
		return
	}
	target, status := pullTarget(activity, items, previous, observed)
	if status == "ambiguous" {
		result.Ambiguous++
		return
	}
	if status != "matched" {
		result.Unmatched++
		return
	}
	result.Matched++
	state := catalog.PlaybackState{Seconds: activity.Seconds, Watched: activity.Watched, Updated: activity.Updated}
	if observed {
		state.Updated = now
	}
	destination := manager.config.Progress(profile, target.ID)
	switch {
	case SameState(destination, state):
		result.Unchanged++
	case Conflict(destination, state):
		result.Conflicts++
	default:
		*changes = append(*changes, ProgressChange{TargetID: target.ID, State: state, Expected: destination})
		result.Importable++
	}
	seen[key] = Observation{TargetID: target.ID, Signature: signature}
}

func pullTarget(activity Activity, items []library.Item, previous Observation, observed bool) (library.Item, string) {
	target, status, _ := Match(activity, items)
	if observed {
		if prior, exists := findLibraryItem(items, previous.TargetID); exists {
			return prior, "matched"
		}
	}
	return target, status
}

func findLibraryItem(items []library.Item, id string) (library.Item, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return library.Item{}, false
}

func (manager *Manager) Syncs() []SyncView {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	result := make([]SyncView, 0, len(manager.syncs))
	for _, sync := range manager.syncs {
		result = append(result, manager.viewLocked(sync))
	}
	slices.SortFunc(result, func(left, right SyncView) int { return strings.Compare(left.ID, right.ID) })
	return result
}

func (manager *Manager) viewLocked(sync Sync) SyncView {
	profile, _ := manager.config.Profile(sync.ProfileID)
	view := SyncView{ID: sync.ID, Source: sync.Source, URL: sync.URL, SourceUser: sync.SourceUser, ProfileID: sync.ProfileID, ProfileName: profile.Name, Interval: sync.Interval, LastError: sync.LastError, LastResult: sync.LastResult, Running: sync.Running}
	if !sync.LastRun.IsZero() {
		view.LastRun = sync.LastRun.Local().Format("Jan 2, 3:04 PM")
	}
	if !sync.NextRun.IsZero() {
		view.NextRun = sync.NextRun.Local().Format("Jan 2, 3:04 PM")
	}
	return view
}

func (manager *Manager) RemoveSync(id string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, found := manager.syncs[id]; !found {
		return errors.New("viewing activity sync was not found")
	}
	syncs := CloneSyncs(manager.syncs)
	delete(syncs, id)
	if err := manager.save(syncs); err != nil {
		return err
	}
	manager.syncs = syncs
	return nil
}

func (manager *Manager) save(syncs map[string]Sync) error {
	if manager.loadErr != nil {
		return errors.New("viewing activity sync state is unavailable")
	}
	if manager.file == "" {
		return errors.New("viewing activity sync storage is not configured")
	}
	if manager.config.Persist == nil {
		return errors.New("viewing activity sync storage is not configured")
	}
	return manager.config.Persist(manager.file, syncs)
}
