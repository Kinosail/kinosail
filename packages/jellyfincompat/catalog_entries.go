package jellyfincompat

import (
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type catalogEntry struct {
	kind, lower, raw string
	item             *library.Item
	show             *library.Show
	season           int
}

func (entry catalogEntry) created() string {
	return entry.item.Added.UTC().Format(time.RFC3339Nano)
}

func itemEntry(item *library.Item) catalogEntry {
	kind, _, _ := itemClass(*item)
	return catalogEntry{kind: kind, lower: strings.ToLower(item.Title), raw: RawID(ID(item.ID)), item: item}
}

func seriesEntry(show *library.Show) catalogEntry {
	return catalogEntry{kind: "Series", lower: strings.ToLower(show.Title), raw: RawID(ID(show.ID)), show: show}
}

func seasonEntry(show *library.Show, season int) catalogEntry {
	name := "Season " + strconv.Itoa(season)
	return catalogEntry{kind: "Season", lower: strings.ToLower(name), raw: RawID(SeasonID(show.ID, season)), show: show, season: season}
}

func appendItemEntries(entries []catalogEntry, items []library.Item) []catalogEntry {
	for position := range items {
		entries = append(entries, itemEntry(&items[position]))
	}
	return entries
}

func movieEntries(view *library.View) []catalogEntry {
	movies := view.Movies()
	return appendItemEntries(make([]catalogEntry, 0, len(movies)), movies)
}

func seriesEntries(view *library.View) []catalogEntry {
	shows := view.Shows()
	entries := make([]catalogEntry, 0, len(shows))
	for position := range shows {
		entries = append(entries, seriesEntry(&shows[position]))
	}
	return entries
}

func episodeEntries(view *library.View) []catalogEntry {
	entries := make([]catalogEntry, 0)
	shows := view.Shows()
	for position := range shows {
		entries = appendItemEntries(entries, shows[position].Episodes)
	}
	return entries
}

func rootEntries(view *library.View) []catalogEntry {
	items := view.Items()
	entries := append(movieEntries(view), seriesEntries(view)...)
	for position := range items {
		if items[position].Kind != "video" {
			entries = append(entries, itemEntry(&items[position]))
		}
	}
	return entries
}
