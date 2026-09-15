package dashboard

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

const boardDocument = "board.json"

var (
	ErrConflict = errors.New("the board changed; refresh and try again")
	ErrNotFound = errors.New("application was not found")
	ErrState    = errors.New("dashboard state is unavailable")
)

type documentStore interface {
	LoadJSON(context.Context, string, any) (bool, error)
	SaveJSON(context.Context, string, any) error
}

// Service owns all board mutations for the web, API, and MCP adapters.
type Service struct {
	mu               sync.RWMutex
	store            documentStore
	board            Board
	health           map[string]Health
	healthGeneration map[string]uint64
	probeRevision    map[string]uint64
	now              func() time.Time
	randomID         func() (string, error)
}

// NewService loads and validates the durable board.
func NewService(ctx context.Context, store documentStore) (*Service, error) {
	if ctx == nil || store == nil {
		return nil, errors.New("dashboard state is required")
	}
	service := &Service{store: store, health: make(map[string]Health), healthGeneration: make(map[string]uint64), probeRevision: make(map[string]uint64), now: time.Now, randomID: newID}
	found, err := store.LoadJSON(ctx, boardDocument, &service.board)
	if err != nil {
		return nil, err
	}
	if !found {
		service.board = Board{Title: "Home", Version: 1, Apps: []App{}, Audit: []AuditEvent{}, UpdatedAt: service.now().UTC()}
		if err := store.SaveJSON(ctx, boardDocument, service.board); err != nil {
			return nil, err
		}
	}
	if err := validatePersistedBoard(service.board, service.now().UTC()); err != nil {
		return nil, err
	}
	return service, nil
}

// Snapshot returns isolated durable configuration and current observations.
func (service *Service) Snapshot() Snapshot {
	service.mu.RLock()
	defer service.mu.RUnlock()
	result := Snapshot{Title: service.board.Title, Version: service.board.Version, Audit: append([]AuditEvent(nil), service.board.Audit...), UpdatedAt: service.board.UpdatedAt}
	result.Removed = append([]RemovedApp{}, service.board.Removed...)
	result.Apps = make([]AppView, 0, len(service.board.Apps))
	for _, app := range service.board.Apps {
		health, found := service.health[app.ID]
		switch {
		case !app.CheckEnabled:
			health = Health{State: "disabled", Explanation: "Checks are off"}
		case !found:
			health = Health{State: "unchecked", Explanation: "Not checked yet"}
		case !health.CheckedAt.IsZero() && service.now().Sub(health.CheckedAt) > 2*time.Minute:
			health.State, health.Explanation = "unchecked", "Last observation is stale"
		}
		result.Apps = append(result.Apps, AppView{App: app, Health: health})
		incrementSummary(&result.Summary, health.State)
	}
	return result
}

// BoardExport returns a copy suitable for backup without session data.
func (service *Service) BoardExport() Board {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return cloneBoard(service.board)
}

// Create adds one validated application at the end of the board.
func (service *Service) Create(ctx context.Context, input CreateInput, actor string) (App, Receipt, error) {
	app, err := normalizeCreate(input)
	if err != nil {
		return App{}, Receipt{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(input.ExpectedVersion); err != nil {
		return App{}, Receipt{}, err
	}
	if len(service.board.Apps) >= MaxApps {
		return App{}, Receipt{}, fmt.Errorf("board supports at most %d applications", MaxApps)
	}
	if duplicateURL(service.board.Apps, app.URL, "") {
		return App{}, Receipt{}, errors.New("an application with this address already exists")
	}
	app.ID, err = service.randomID()
	if err != nil {
		return App{}, Receipt{}, ErrState
	}
	now := service.now().UTC()
	app.CreatedAt, app.UpdatedAt = now, now
	before := service.board.Version
	service.board.Apps = append(service.board.Apps, app)
	service.record("app.added", app.Name, actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board.Apps = service.board.Apps[:len(service.board.Apps)-1]
		service.rollbackAudit()
		return App{}, Receipt{}, err
	}
	return app, receipt("app.added", app.ID, app.Name+" was added", before, service.board.Version), nil
}

// Update changes one application after validating the complete result.
func (service *Service) Update(ctx context.Context, id string, input UpdateInput, actor string) (App, Receipt, error) { //nolint:cyclop // Pointer fields form one explicit patch contract.
	if !validID(id) || noUpdate(input) {
		return App{}, Receipt{}, errors.New("application update is invalid")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(input.ExpectedVersion); err != nil {
		return App{}, Receipt{}, err
	}
	index := slices.IndexFunc(service.board.Apps, func(app App) bool { return app.ID == id })
	if index < 0 {
		return App{}, Receipt{}, ErrNotFound
	}
	previous := service.board.Apps[index]
	candidate := CreateInput{Name: previous.Name, URL: previous.URL, HealthURL: previous.HealthURL, CheckEnabled: previous.CheckEnabled, Description: previous.Description, Category: previous.Category, Icon: previous.Icon, Accent: previous.Accent, Favorite: previous.Favorite}
	applyUpdate(&candidate, input)
	if input.URL != nil && input.HealthURL == nil && previous.HealthURL == previous.URL {
		candidate.HealthURL = candidate.URL
	}
	updated, err := normalizeCreate(candidate)
	if err != nil {
		return App{}, Receipt{}, err
	}
	if duplicateURL(service.board.Apps, updated.URL, id) {
		return App{}, Receipt{}, errors.New("an application with this address already exists")
	}
	updated.ID, updated.CreatedAt, updated.UpdatedAt = previous.ID, previous.CreatedAt, service.now().UTC()
	before := service.board.Version
	service.board.Apps[index] = updated
	service.record("app.updated", updated.Name, actor, updated.UpdatedAt)
	if err := service.commit(ctx, updated.UpdatedAt); err != nil {
		service.board.Apps[index] = previous
		service.rollbackAudit()
		return App{}, Receipt{}, err
	}
	service.invalidateHealth(updated.ID)
	return updated, receipt("app.updated", id, updated.Name+" was updated", before, service.board.Version), nil
}

// Remove moves one application into the bounded recovery list.
func (service *Service) Remove(ctx context.Context, id string, expected uint64, actor string) (Receipt, error) {
	if !validID(id) {
		return Receipt{}, ErrNotFound
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(expected); err != nil {
		return Receipt{}, err
	}
	index := slices.IndexFunc(service.board.Apps, func(app App) bool { return app.ID == id })
	if index < 0 {
		return Receipt{}, ErrNotFound
	}
	now, before := service.now().UTC(), service.board.Version
	removed := RemovedApp{App: service.board.Apps[index], Position: index, RemovedAt: now}
	appsBefore, removedBefore := append([]App(nil), service.board.Apps...), append([]RemovedApp(nil), service.board.Removed...)
	service.board.Apps = slices.Delete(service.board.Apps, index, index+1)
	service.board.Removed = append([]RemovedApp{removed}, service.board.Removed...)
	if len(service.board.Removed) > 10 {
		service.board.Removed = service.board.Removed[:10]
	}
	service.record("app.removed", removed.App.Name, actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board.Apps, service.board.Removed = appsBefore, removedBefore
		service.rollbackAudit()
		return Receipt{}, err
	}
	service.invalidateHealth(id)
	return receipt("app.removed", id, removed.App.Name+" was removed; it can be restored", before, service.board.Version), nil
}

// Restore returns a recently removed application to its prior position.
func (service *Service) Restore(ctx context.Context, id string, expected uint64, actor string) (App, Receipt, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(expected); err != nil {
		return App{}, Receipt{}, err
	}
	index := slices.IndexFunc(service.board.Removed, func(item RemovedApp) bool { return item.App.ID == id })
	if index < 0 {
		return App{}, Receipt{}, ErrNotFound
	}
	item, now, before := service.board.Removed[index], service.now().UTC(), service.board.Version
	if len(service.board.Apps) >= MaxApps || duplicateURL(service.board.Apps, item.App.URL, "") {
		return App{}, Receipt{}, errors.New("removed application conflicts with the current board")
	}
	appsBefore, removedBefore := append([]App(nil), service.board.Apps...), append([]RemovedApp(nil), service.board.Removed...)
	position := min(item.Position, len(service.board.Apps))
	service.board.Apps = slices.Insert(service.board.Apps, position, item.App)
	service.board.Removed = slices.Delete(service.board.Removed, index, index+1)
	service.record("app.restored", item.App.Name, actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board.Apps, service.board.Removed = appsBefore, removedBefore
		service.rollbackAudit()
		return App{}, Receipt{}, err
	}
	service.invalidateHealth(id)
	return item.App, receipt("app.restored", id, item.App.Name+" was restored", before, service.board.Version), nil
}

// Reorder atomically applies one exact permutation of all application IDs.
func (service *Service) Reorder(ctx context.Context, ids []string, expected uint64, actor string) (Receipt, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(expected); err != nil {
		return Receipt{}, err
	}
	if len(ids) != len(service.board.Apps) || len(ids) > MaxApps {
		return Receipt{}, errors.New("order must contain every application exactly once")
	}
	byID := make(map[string]App, len(service.board.Apps))
	for _, app := range service.board.Apps {
		byID[app.ID] = app
	}
	ordered := make([]App, 0, len(ids))
	for _, id := range ids {
		app, found := byID[id]
		if !found {
			return Receipt{}, errors.New("order contains an unknown or repeated application")
		}
		ordered = append(ordered, app)
		delete(byID, id)
	}
	before, previous, now := service.board.Version, service.board.Apps, service.now().UTC()
	service.board.Apps = ordered
	service.record("board.reordered", fmt.Sprintf("%d applications", len(ids)), actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board.Apps = previous
		service.rollbackAudit()
		return Receipt{}, err
	}
	return receipt("board.reordered", "", "Application order was saved", before, service.board.Version), nil
}

// Rename changes the one board title.
func (service *Service) Rename(ctx context.Context, title string, expected uint64, actor string) (Receipt, error) {
	title, err := boundedText("title", title, 1, 60)
	if err != nil {
		return Receipt{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(expected); err != nil {
		return Receipt{}, err
	}
	previous, before, now := service.board.Title, service.board.Version, service.now().UTC()
	service.board.Title = title
	service.record("board.renamed", title, actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board.Title = previous
		service.rollbackAudit()
		return Receipt{}, err
	}
	return receipt("board.renamed", "", "Board name was saved", before, service.board.Version), nil
}
