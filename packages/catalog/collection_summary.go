package catalog

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// CollectionSummary is the canonical collection card and API projection.
type CollectionSummary struct {
	Name       string         `json:"name"`
	Source     string         `json:"source"`
	ItemCount  int            `json:"itemCount"`
	ItemLabel  string         `json:"-"`
	ArtworkIDs []string       `json:"artworkIds"`
	Preview    []library.Item `json:"-"`
}

// Path keeps imported metadata names within a single route segment.
func (summary CollectionSummary) Path() string { return "/collection/" + url.PathEscape(summary.Name) }

// CollectionSummaries projects visible collections and their first four members.
func (storage ListStorage) CollectionSummaries(items []library.Item, query string) []CollectionSummary {
	summaries := make([]CollectionSummary, 0)
	defaults := make(map[string]bool)
	for _, item := range items {
		if item.Collection != "" {
			defaults[item.Collection] = true
		}
	}
	for _, name := range storage.CollectionNames(items) {
		members := storage.Collection(name, items)
		if !CollectionMatches(name, members, query) {
			continue
		}
		source := "custom"
		if defaults[name] {
			source = "default"
		}
		summary := CollectionSummary{Name: name, Source: source, ItemCount: len(members), ItemLabel: CountLabel(len(members)), ArtworkIDs: []string{}}
		summary.Preview, summary.ArtworkIDs = summaryPreview(members)
		summaries = append(summaries, summary)
	}
	return summaries
}

// CollectionMatches searches a collection name and its visible member metadata.
func CollectionMatches(name string, items []library.Item, query string) bool {
	if query == "" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
		return true
	}
	for _, item := range items {
		if Matches(item, query) {
			return true
		}
	}
	return false
}

// CountLabel returns Player's singular or plural item count.
func CountLabel(count int) string {
	if count == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", count)
}

func summaryPreview(members []library.Item) ([]library.Item, []string) {
	var preview []library.Item
	artworkIDs := make([]string, 0)
	for _, item := range members[:min(4, len(members))] {
		preview = append(preview, item)
		if item.Artwork != "" || item.ShowArtwork != "" {
			artworkIDs = append(artworkIDs, item.ID)
		}
	}
	return preview, artworkIDs
}
