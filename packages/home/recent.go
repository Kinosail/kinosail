package home

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const recentItemLimit = 12

type MovieGenre struct {
	Name  string
	Items []ShelfItem
}

type HomeShelf struct {
	Key    string
	Title  string
	Items  []ShelfItem
	Genres []MovieGenre
}

func newestFirst(items []library.Item, kind string) []*library.Item {
	matched := make([]*library.Item, 0)
	for position := range items {
		if homeKind(items[position], kind) {
			matched = append(matched, &items[position])
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Added.After(matched[j].Added) })
	return matched
}

func recentShelf(newest []*library.Item, artwork, showTitles map[string]string, include func(library.Item) bool) []ShelfItem {
	selected := make([]library.Item, 0, recentItemLimit)
	for _, item := range newest {
		if len(selected) == recentItemLimit {
			break
		}
		if include(*item) {
			selected = append(selected, *item)
		}
	}
	return episodeShelf(selected, artwork, showTitles, "play", "recently added episodes stacked")
}

func homeKind(item library.Item, kind string) bool {
	switch kind {
	case "movie":
		return item.Kind == "video" && item.Show == ""
	case "show":
		return item.Kind == "video" && item.Show != ""
	case "other":
		return item.Kind != "video" && item.Kind != "audio" && item.Kind != "audiobook"
	default:
		return item.Kind == kind
	}
}

func MovieGenres(items []library.Item, artwork map[string]string) []MovieGenre {
	grouped := make(map[string][]library.Item)
	for _, item := range items {
		if item.Kind != "video" || item.Show != "" {
			continue
		}
		seen := make(map[string]bool)
		for _, part := range strings.Split(item.Genres, " · ") {
			name := strings.TrimSpace(part)
			if name != "" && !seen[name] {
				grouped[name] = append(grouped[name], item)
				seen[name] = true
			}
		}
	}
	genres := make([]MovieGenre, 0, len(grouped))
	for name, movies := range grouped {
		sort.SliceStable(movies, func(i, j int) bool { return movies[i].Added.After(movies[j].Added) })
		cards := make([]ShelfItem, 0, min(len(movies), recentItemLimit))
		for _, movie := range movies[:min(len(movies), recentItemLimit)] {
			cards = append(cards, singleItem(movie, artwork, nil, "play"))
		}
		genres = append(genres, MovieGenre{Name: name, Items: cards})
	}
	sort.Slice(genres, func(i, j int) bool {
		if len(grouped[genres[i].Name]) == len(grouped[genres[j].Name]) {
			return genres[i].Name < genres[j].Name
		}
		return len(grouped[genres[i].Name]) > len(grouped[genres[j].Name])
	})
	return genres[:min(len(genres), 4)]
}

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
	return episodeShelf(selected, artwork, showTitles, "play", "recently added episodes stacked")
}

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
			stack.PlayHref = ""
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

var kindPlaceholderIcons = map[string]string{"audio": "music", "audiobook": "audiobook", "book": "book", "photo": "photo"}

func singleItem(item library.Item, artwork, showTitles map[string]string, placeholderIcon string) ShelfItem {
	title := item.Title
	if showTitle := showTitles[item.ID]; showTitle != "" {
		title = showTitle + " · " + title
	}
	if icon := kindPlaceholderIcons[item.Kind]; icon != "" {
		placeholderIcon = icon
	}
	card := ShelfItem{Href: mediaHref(item), Title: title, Meta: mediaMeta(item), ArtworkID: artwork[item.ID], PlaceholderIcon: placeholderIcon, Count: 1}
	if item.Kind == "video" {
		card.PlayHref = card.Href
	}
	return card
}

func mediaHref(item library.Item) string {
	if item.Kind == "book" {
		return "/book/" + item.ID
	}
	return "/watch/" + item.ID
}

func mediaMeta(item library.Item) string {
	genre, _, _ := strings.Cut(item.Genres, " · ")
	switch {
	case item.Year == "":
		return strings.TrimSpace(genre)
	case strings.TrimSpace(genre) == "":
		return item.Year
	}
	return item.Year + " · " + strings.TrimSpace(genre)
}

func stackMeta(count int, item library.Item) string {
	meta := fmt.Sprintf("%d episodes stacked", count)
	if detail := mediaMeta(item); detail != "" {
		meta += " · " + detail
	}
	return meta
}
