package home

import (
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// Destination is one nonempty home Library category.
type Destination struct {
	Name, Description, Href, Icon, CountLabel string
}

// DestinationGroup is one related set of Library categories.
type DestinationGroup struct {
	Name         string
	Destinations []Destination
}

// Destinations projects Player-canonical home links and wording.
func Destinations(items []library.Item, listCount, collectionCount, playlistCount int) []DestinationGroup {
	movies, shows := library.Organize(items)
	groups := []DestinationGroup{
		{"Watch & view", []Destination{{"Movies", "Feature films", "/?view=movies", "play", CountLabel(len(movies))}, {"Shows", "Series and episodes", "/?view=shows", "shows", CountLabel(len(shows))}, {"Photos", "Photo library", "/?view=photos", "photo", CountLabel(len(Media(items, "photo")))}}},
		{"Listen & read", []Destination{{"Music", "Albums and tracks", "/?view=music", "music", CountLabel(len(Media(items, "audio")))}, {"Audiobooks", "Long-form listening", "/?view=audiobooks", "audiobook", CountLabel(len(Media(items, "audiobook")))}, {"Books", "Books and comics", "/?view=books", "book", CountLabel(len(Media(items, "book")))}}},
		{"Organize", []Destination{{"My List", "Saved for later", "/?view=list", "star", CountLabel(listCount)}, {"Collections", "Curated worlds", "/?view=collections", "collection", CountLabel(collectionCount)}, {"Playlists", "Queues and smart mixes", "/?view=playlists", "playlist", CountLabel(playlistCount)}}},
	}
	visible := groups[:0]
	for _, group := range groups {
		group.Destinations = nonemptyDestinations(group.Destinations)
		if len(group.Destinations) > 0 {
			visible = append(visible, group)
		}
	}
	return visible
}

func nonemptyDestinations(destinations []Destination) []Destination {
	visible := destinations[:0]
	for _, destination := range destinations {
		if destination.CountLabel != "0 items" {
			visible = append(visible, destination)
		}
	}
	return visible
}

// CountLabel returns the Player-canonical singular or plural item count.
func CountLabel(count int) string {
	return catalog.CountLabel(count)
}
