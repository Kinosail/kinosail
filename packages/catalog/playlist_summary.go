package catalog

import (
	"fmt"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// PlaylistSummary is the canonical playlist card and API projection.
type PlaylistSummary struct {
	Name       string         `json:"name"`
	ItemCount  int            `json:"itemCount"`
	ArtworkIDs []string       `json:"artworkIds"`
	Mode       string         `json:"mode"`
	Rule       string         `json:"rule,omitempty"`
	Preview    []library.Item `json:"-"`
}

// PlaylistSummaries projects the viewer's playlists and their first four members.
func (storage ListStorage) PlaylistSummaries(viewer string, items []library.Item, query string) []PlaylistSummary {
	summaries := make([]PlaylistSummary, 0)
	for _, name := range storage.PlaylistNames(viewer) {
		members := storage.Playlist(viewer, name, items)
		if !CollectionMatches(name, members, query) {
			continue
		}
		rule, smart := storage.PlaylistRule(viewer, name)
		summary := PlaylistSummary{Name: name, ItemCount: len(members), ArtworkIDs: []string{}, Mode: "Manual"}
		if smart {
			summary.Mode, summary.Rule = "Smart", DescribePlaylistRule(rule)
		}
		summary.Preview, summary.ArtworkIDs = summaryPreview(members)
		summaries = append(summaries, summary)
	}
	return summaries
}

// PlaylistRule returns a viewer's stored smart-list rule.
func (storage ListStorage) PlaylistRule(viewer, name string) (PlaylistRule, bool) {
	rule, found := (*storage.Smart)[viewer+":"+name]
	return rule, found
}

// DescribePlaylistRule returns Player's user-facing smart-list description.
func DescribePlaylistRule(rule PlaylistRule) string {
	parts := make([]string, 0, 2)
	if rule.Query != "" {
		parts = append(parts, fmt.Sprintf("matching %q", rule.Query))
	}
	if rule.Kind != "" {
		parts = append(parts, rule.Kind)
	}
	if len(parts) == 0 {
		return "All media"
	}
	return strings.Join(parts, " · ")
}
