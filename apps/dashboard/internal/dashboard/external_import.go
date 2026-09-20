package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const MaxExternalImportSize = 2 << 20

// ExternalImport is a previewable import from a supported dashboard format.
type ExternalImport struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

// ImportedApp is an uncommitted, normalized application candidate.
type ImportedApp struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Description  string `json:"description,omitempty"`
	Category     string `json:"category,omitempty"`
	Icon         string `json:"icon"`
	Accent       string `json:"accent"`
	CheckEnabled bool   `json:"checkEnabled"`
}

// ImportPreview describes what will be added before a board replacement.
type ImportPreview struct {
	Source   string        `json:"source"`
	Title    string        `json:"title"`
	Apps     []ImportedApp `json:"apps"`
	Warnings []string      `json:"warnings,omitempty"`
}

// PreviewExternal parses a supported config without network or state changes.
func PreviewExternal(input ExternalImport) (ImportPreview, error) {
	source, err := externalSource(input)
	if err != nil {
		return ImportPreview{}, err
	}
	value, err := decodeConfig(input.Content)
	if err != nil {
		return ImportPreview{}, errors.New("import file is not valid JSON or YAML")
	}
	title, apps := externalApps(source, value)
	preview := ImportPreview{Source: source, Title: title, Apps: apps}
	if title := stringValue(mapValue(value, "pageInfo"), "title"); title != "" {
		preview.Title = title
	}
	preview.Title, err = boundedText("title", preview.Title, 1, 60)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(preview.Apps) == 0 {
		return ImportPreview{}, errors.New("no direct application links were found")
	}
	if len(preview.Apps) > MaxApps {
		return ImportPreview{}, fmt.Errorf("import contains more than %d applications", MaxApps)
	}
	preview.Warnings = []string{"Imported addresses are saved directly; credentials, widgets, and integrations are ignored."}
	return preview, nil
}

func externalSource(input ExternalImport) (string, error) {
	if len(input.Content) == 0 || len(input.Content) > MaxExternalImportSize {
		return "", errors.New("import content must be between 1 byte and 2 MiB")
	}
	source := strings.ToLower(strings.TrimSpace(input.Source))
	if source != "homarr" && source != "homepage" && source != "dashy" {
		return "", errors.New("import source is not supported")
	}
	return source, nil
}

func externalApps(source string, value any) (string, []ImportedApp) {
	switch source {
	case "dashy":
		return dashyApps(value)
	case "homepage":
		return "Home", homepageApps(value)
	default:
		return "Home", genericApps(value)
	}
}

// ImportExternal parses and merges a config into the board after preview.
func (service *Service) ImportExternal(ctx context.Context, input ExternalImport, expected uint64, actor string) (Receipt, error) {
	preview, err := PreviewExternal(input)
	if err != nil {
		return Receipt{}, err
	}
	now := service.now().UTC()
	apps, err := service.normalizeImportedApps(preview.Apps, now)
	if err != nil {
		return Receipt{}, err
	}
	if len(apps) == 0 {
		return Receipt{}, errors.New("import contains no valid HTTP or HTTPS applications")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.canMutate(expected); err != nil {
		return Receipt{}, err
	}
	previous, before := cloneBoard(service.board), service.board.Version
	seenExisting := make(map[string]bool, len(service.board.Apps))
	for _, app := range service.board.Apps {
		seenExisting[strings.ToLower(app.URL)] = true
	}
	added := mergeImportedApps(service.board.Apps, apps, seenExisting)
	if len(added) == 0 {
		return Receipt{}, errors.New("all imported applications already exist or the board is full")
	}
	service.board.Apps = append(service.board.Apps, added...)
	if len(previous.Apps) == 0 {
		service.board.Title = preview.Title
	}
	service.record("board.imported", fmt.Sprintf("%d applications from %s", len(added), preview.Source), actor, now)
	if err := service.commit(ctx, now); err != nil {
		service.board = previous
		return Receipt{}, err
	}
	for _, app := range added {
		service.invalidateHealth(app.ID)
	}
	return receipt("board.imported", "", fmt.Sprintf("Imported %d applications from %s", len(added), preview.Source), before, service.board.Version), nil
}

func (service *Service) normalizeImportedApps(candidates []ImportedApp, now time.Time) ([]App, error) {
	apps := make([]App, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		app, err := normalizeCreate(CreateInput{Name: candidate.Name, URL: candidate.URL, CheckEnabled: candidate.CheckEnabled, Description: candidate.Description, Category: candidate.Category, Icon: candidate.Icon, Accent: candidate.Accent})
		if err != nil || seen[strings.ToLower(app.URL)] {
			continue
		}
		id, err := service.randomID()
		if err != nil {
			return nil, ErrState
		}
		app.ID, app.CreatedAt, app.UpdatedAt = id, now, now
		seen[strings.ToLower(app.URL)] = true
		apps = append(apps, app)
	}
	return apps, nil
}

func mergeImportedApps(existing, candidates []App, seen map[string]bool) []App {
	added := make([]App, 0, len(candidates))
	for _, app := range candidates {
		if seen[strings.ToLower(app.URL)] || len(existing)+len(added) >= MaxApps {
			continue
		}
		seen[strings.ToLower(app.URL)] = true
		added = append(added, app)
	}
	return added
}
