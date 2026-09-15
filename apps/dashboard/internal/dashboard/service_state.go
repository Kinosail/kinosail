package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
)

func (service *Service) canMutate(expected uint64) error {
	if expected == 0 {
		return errors.New("expectedVersion is required")
	}
	if expected != service.board.Version {
		return ErrConflict
	}
	return nil
}

func (service *Service) commit(ctx context.Context, now time.Time) error {
	previousVersion, previousUpdated := service.board.Version, service.board.UpdatedAt
	service.board.Version++
	service.board.UpdatedAt = now
	if err := service.store.SaveJSON(context.WithoutCancel(ctx), boardDocument, service.board); err != nil {
		service.board.Version, service.board.UpdatedAt = previousVersion, previousUpdated
		return ErrState
	}
	return nil
}

func (service *Service) record(action, subject, actor string, at time.Time) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "Owner"
	}
	service.board.Audit = append([]AuditEvent{{Action: action, Subject: subject, Actor: actor, At: at}}, service.board.Audit...)
	if len(service.board.Audit) > 100 {
		service.board.Audit = service.board.Audit[:100]
	}
}

func (service *Service) rollbackAudit() {
	if len(service.board.Audit) > 0 {
		service.board.Audit = service.board.Audit[1:]
	}
}

func (service *Service) invalidateHealth(id string) {
	service.healthGeneration[id]++
	service.probeRevision[id]++
	delete(service.health, id)
}

func applyUpdate(target *CreateInput, update UpdateInput) {
	if update.Name != nil {
		target.Name = *update.Name
	}
	if update.URL != nil {
		target.URL = *update.URL
	}
	if update.HealthURL != nil {
		target.HealthURL = *update.HealthURL
	}
	if update.CheckEnabled != nil {
		target.CheckEnabled = *update.CheckEnabled
	}
	if update.Description != nil {
		target.Description = *update.Description
	}
	if update.Category != nil {
		target.Category = *update.Category
	}
	if update.Icon != nil {
		target.Icon = *update.Icon
	}
	if update.Accent != nil {
		target.Accent = *update.Accent
	}
	if update.Favorite != nil {
		target.Favorite = *update.Favorite
	}
}

func noUpdate(input UpdateInput) bool {
	return input.Name == nil && input.URL == nil && input.HealthURL == nil && input.CheckEnabled == nil && input.Description == nil && input.Category == nil && input.Icon == nil && input.Accent == nil && input.Favorite == nil
}

func duplicateURL(apps []App, address, except string) bool {
	return slices.ContainsFunc(apps, func(app App) bool { return app.ID != except && strings.EqualFold(app.URL, address) })
}

func validatePersistedBoard(board Board, now time.Time) error {
	if _, err := boundedText("title", board.Title, 1, 60); err != nil || board.Version == 0 || len(board.Apps) > MaxApps || len(board.Removed) > 10 || len(board.Audit) > 100 {
		return errors.New("persisted board is invalid")
	}
	seenIDs, seenURLs := map[string]bool{}, map[string]bool{}
	if validatePersistedApps(board.Apps, now, seenIDs, seenURLs) != nil || validateRemovedApps(board.Removed, now, seenIDs) != nil || validateAudit(board.Audit) != nil {
		return errors.New("persisted board is invalid")
	}
	return nil
}

func validatePersistedApps(apps []App, now time.Time, seenIDs, seenURLs map[string]bool) error {
	for _, app := range apps {
		candidate, err := normalizeCreate(CreateInput{Name: app.Name, URL: app.URL, HealthURL: app.HealthURL, CheckEnabled: app.CheckEnabled, Description: app.Description, Category: app.Category, Icon: app.Icon, Accent: app.Accent, Favorite: app.Favorite})
		urlKey := strings.ToLower(candidate.URL)
		if err != nil || !validID(app.ID) || !validAppTimestamps(app.CreatedAt, app.UpdatedAt, now) || seenIDs[app.ID] || seenURLs[urlKey] {
			return errors.New("persisted board contains an invalid application")
		}
		seenIDs[app.ID], seenURLs[urlKey] = true, true
	}
	return nil
}

func validateRemovedApps(items []RemovedApp, now time.Time, seenIDs map[string]bool) error {
	for _, item := range items {
		_, err := normalizeCreate(CreateInput{Name: item.App.Name, URL: item.App.URL, HealthURL: item.App.HealthURL, CheckEnabled: item.App.CheckEnabled, Description: item.App.Description, Category: item.App.Category, Icon: item.App.Icon, Accent: item.App.Accent, Favorite: item.App.Favorite})
		if err != nil || !validID(item.App.ID) || !validAppTimestamps(item.App.CreatedAt, item.App.UpdatedAt, now) || item.RemovedAt.Before(item.App.UpdatedAt) || item.RemovedAt.After(now.Add(5*time.Minute)) || item.Position < 0 || item.Position > MaxApps || seenIDs[item.App.ID] {
			return errors.New("persisted board contains an invalid removed application")
		}
		seenIDs[item.App.ID] = true
	}
	return nil
}

func validateAudit(events []AuditEvent) error {
	for _, event := range events {
		if !strings.Contains(event.Action, ".") || event.At.IsZero() {
			return errors.New("persisted board contains an invalid audit event")
		}
		if _, err := boundedText("audit subject", event.Subject, 1, 120); err != nil {
			return errors.New("persisted board contains an invalid audit event")
		}
		if _, err := boundedText("audit actor", event.Actor, 1, 80); err != nil {
			return errors.New("persisted board contains an invalid audit event")
		}
	}
	return nil
}

func validAppTimestamps(created, updated, now time.Time) bool {
	lowerBound := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	return !created.Before(lowerBound) && !updated.Before(created) && !updated.After(now.Add(5*time.Minute))
}

func validID(id string) bool { return len(id) == 24 && tokenPattern.MatchString(id) }

func newID() (string, error) {
	var data [12]byte
	_, _ = rand.Read(data[:]) // Go guarantees success or terminates the process.
	return hex.EncodeToString(data[:]), nil
}

func cloneBoard(board Board) Board {
	board.Apps = append([]App(nil), board.Apps...)
	board.Removed = append([]RemovedApp(nil), board.Removed...)
	board.Audit = append([]AuditEvent(nil), board.Audit...)
	return board
}

func receipt(action, appID, message string, before, after uint64) Receipt {
	return Receipt{Action: action, AppID: appID, BeforeVersion: before, AfterVersion: after, Message: message}
}

func incrementSummary(summary *Summary, state string) {
	summary.Total++
	switch state {
	case "reachable":
		summary.Reachable++
	case "slow":
		summary.Slow++
	case "degraded":
		summary.Degraded++
	case "unavailable":
		summary.Unavailable++
	case "disabled":
		summary.Disabled++
	default:
		summary.Unchecked++
	}
}
