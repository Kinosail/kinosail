package home

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const recentItemLimit = 12

// MovieGenre is one populated movie shelf on Home.
type MovieGenre struct {
	Name  string
	Items []ShelfItem
}

// HomeShelf is one vertically stacked media group on the Player home page.
type HomeShelf struct {
	Key    string
	Title  string
	Items  []ShelfItem
	Genres []MovieGenre
}

// HomeShelves follows the native watch-home order while retaining non-video media.
func HomeShelves(items, unwatched []library.Item, artwork, showTitles map[string]string, genres []MovieGenre) []HomeShelf {
	shelves := []HomeShelf{
		{Key: "recent-movies", Title: "Recently added movies", Items: recentFor(items, artwork, showTitles, "movie")},
		{Key: "recent-shows", Title: "Recently added TV shows", Items: recentFor(items, artwork, showTitles, "show")},
		{Key: "unwatched-shows", Title: "Unwatched TV shows", Items: recentFor(unwatched, artwork, showTitles, "show")},
		{Key: "unwatched-movies", Title: "Unwatched movies", Items: recentFor(unwatched, artwork, showTitles, "movie")},
		{Key: "movie-genres", Title: "Movie genres", Genres: genres},
		{Key: "recent-music", Title: "Recently added music", Items: recentFor(items, artwork, showTitles, "audio")},
		{Key: "recent-audiobooks", Title: "Recently added audiobooks", Items: recentFor(items, artwork, showTitles, "audiobook")},
		{Key: "recent-other", Title: "Recently added", Items: recentFor(items, artwork, showTitles, "other")},
	}
	visible := shelves[:0]
	for _, shelf := range shelves {
		if len(shelf.Items) > 0 || len(shelf.Genres) > 0 {
			visible = append(visible, shelf)
		}
	}
	for index := range visible {
		for card := range visible[index].Items {
			visible[index].Items[card].Priority = index == 0 && card < 2
		}
	}
	return visible
}

func recentFor(items []library.Item, artwork, showTitles map[string]string, kind string) []ShelfItem {
	selected := make([]library.Item, 0, len(items))
	for _, item := range items {
		if homeKind(item, kind) {
			selected = append(selected, item)
		}
	}
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Added.After(selected[j].Added) })
	selected = selected[:min(len(selected), recentItemLimit)]
	cards := episodeShelf(selected, artwork, showTitles, "plus", "recently added episodes stacked")
	if kind == "movie" {
		for index := range cards {
			cards[index].Href = "/item/" + selected[index].ID
		}
	}
	return cards
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

// MovieGenres shows the most populated genres from the visible movie catalog.
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
			card := singleItem(movie, artwork, nil, "play")
			card.Href = "/item/" + movie.ID
			cards = append(cards, card)
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
