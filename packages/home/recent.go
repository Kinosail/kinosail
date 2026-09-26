package home

import (
	"fmt"
	"sort"

	"github.com/MikeO7/kinosail/packages/library"
)

const recentItemLimit = 12

// ShelfItem is one recent-media card on the Player home page.
type ShelfItem struct {
	Href            string
	PlayHref        string
	Title           string
	Meta            string
	ArtworkID       string
	PlaceholderIcon string
	StackLabel      string
	Count           int
	Stacked         bool
	Priority        bool
}

// RecentlyAdded projects the newest canonical home shelf.
func RecentlyAdded(items []library.Item, artwork, showTitles map[string]string) []ShelfItem {
	recent := make([]*library.Item, len(items))
	for position := range items {
		recent[position] = &items[position]
	}
	sort.SliceStable(recent, func(left, right int) bool { return recent[left].Added.After(recent[right].Added) })
	selected := make([]library.Item, min(len(recent), recentItemLimit))
	for position := range selected {
		selected[position] = *recent[position]
	}
	return episodeShelf(selected, artwork, showTitles, "plus", "recently added episodes stacked")
}

// RecentlyPlayed projects the Player-canonical playback-history shelf.
func RecentlyPlayed(items []library.Item, artwork, showTitles map[string]string) []ShelfItem {
	return episodeShelf(items, artwork, showTitles, "play", "recently played episodes stacked")
}

func episodeShelf(items []library.Item, artwork, showTitles map[string]string, placeholderIcon, stackLabel string) []ShelfItem {
	_, shows := library.Organize(items)
	showByEpisode := make(map[string]library.Show)
	for _, show := range shows {
		for _, episode := range show.Episodes {
			showByEpisode[episode.ID] = show
		}
	}

	result := make([]ShelfItem, 0, len(items))
	showPositions := make(map[string]int)
	for _, item := range items {
		show, isEpisode := showByEpisode[item.ID]
		if !isEpisode {
			result = append(result, singleItem(item, artwork, showTitles, placeholderIcon))
			continue
		}
		if position, exists := showPositions[show.ID]; exists {
			stack := &result[position]
			stack.Count++
			stack.Stacked = true
			stack.Href = "/show/" + show.ID
			stack.Title = show.Title
			stack.StackLabel = stackLabel
			stack.Meta = stackMeta(stack.Count, item)
			continue
		}
		showPositions[show.ID] = len(result)
		result = append(result, singleItem(item, artwork, showTitles, placeholderIcon))
	}
	for position := range result {
		if position == 2 {
			break
		}
		result[position].Priority = true
	}
	return result
}

func singleItem(item library.Item, artwork, showTitles map[string]string, placeholderIcon string) ShelfItem {
	title := item.Title
	if showTitle := showTitles[item.ID]; showTitle != "" {
		title = showTitle + " · " + title
	}
	return ShelfItem{Href: mediaHref(item), Title: title, Meta: mediaMeta(item), ArtworkID: artwork[item.ID], PlaceholderIcon: placeholderIcon, Count: 1}
}

func mediaHref(item library.Item) string {
	if item.Kind == "book" {
		return "/book/" + item.ID
	}
	return "/watch/" + item.ID
}

func mediaMeta(item library.Item) string {
	if item.Container == "" {
		return item.Year
	}
	if item.Year == "" {
		return item.Container
	}
	return item.Container + " · " + item.Year
}

func stackMeta(count int, item library.Item) string {
	meta := fmt.Sprintf("%d episodes stacked", count)
	if detail := mediaMeta(item); detail != "" {
		meta += " · " + detail
	}
	return meta
}
