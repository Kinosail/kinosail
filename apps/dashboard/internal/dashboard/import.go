package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ImportBoard validates a complete backup before replacing the board.
func (service *Service) ImportBoard(ctx context.Context, input Import, actor string) (Receipt, error) {
	title, err := boundedText("title", input.Title, 1, 60)
	if err != nil || len(input.Apps) > MaxApps {
		return Receipt{}, errors.New("imported board is invalid")
	}
	validationTime := service.now().UTC()
	normalized, err := normalizeImportedApps(input.Apps, validationTime)
	if err != nil {
		return Receipt{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(input.ExpectedVersion); err != nil {
		return Receipt{}, err
	}
	previous, before, now := cloneBoard(service.board), service.board.Version, validationTime
	service.board.Title, service.board.Apps, service.board.Removed = title, normalized, nil
	service.record("board.imported", fmt.Sprintf("%d applications", len(normalized)), actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board = previous
		return Receipt{}, err
	}
	invalidated := make(map[string]bool, len(previous.Apps)+len(normalized))
	for _, app := range append(previous.Apps, normalized...) {
		if !invalidated[app.ID] {
			invalidated[app.ID] = true
			service.invalidateHealth(app.ID)
		}
	}
	return receipt("board.imported", "", "Board backup was imported", before, service.board.Version), nil
}

func normalizeImportedApps(apps []App, validationTime time.Time) ([]App, error) {
	normalized := make([]App, 0, len(apps))
	seenIDs, seenURLs := map[string]bool{}, map[string]bool{}
	for _, app := range apps {
		candidate, normalizeErr := normalizeCreate(CreateInput{Name: app.Name, URL: app.URL, HealthURL: app.HealthURL, CheckEnabled: app.CheckEnabled, Description: app.Description, Category: app.Category, Icon: app.Icon, Accent: app.Accent, Favorite: app.Favorite})
		if normalizeErr != nil || !validID(app.ID) || seenIDs[app.ID] || seenURLs[strings.ToLower(candidate.URL)] {
			return nil, errors.New("imported board contains an invalid application")
		}
		candidate.ID, candidate.CreatedAt, candidate.UpdatedAt = app.ID, app.CreatedAt, app.UpdatedAt
		if !validAppTimestamps(candidate.CreatedAt, candidate.UpdatedAt, validationTime) {
			return nil, errors.New("imported application timestamps are invalid")
		}
		seenIDs[app.ID], seenURLs[strings.ToLower(candidate.URL)] = true, true
		normalized = append(normalized, candidate)
	}
	return normalized, nil
}
