package home

import (
	"github.com/MikeO7/kinosail/packages/catalog"
)

type Destination struct {
	Name, Description, Href, Icon, CountLabel string
}

type DestinationGroup struct {
	Name         string
	Destinations []Destination
}

func destinations(counts libraryCounts, listCount, collectionCount, playlistCount int) []DestinationGroup {
	groups := []DestinationGroup{
		{"Watch & view", []Destination{{"Movies", "Feature films", "/?view=movies", "play", countLabel(counts.movies)}, {"Shows", "Series and episodes", "/?view=shows", "shows", countLabel(counts.shows)}, {"Photos", "Photo library", "/?view=photos", "photo", countLabel(counts.photos)}}},
		{"Listen & read", []Destination{{"Music", "Albums and tracks", "/?view=music", "music", countLabel(counts.music)}, {"Audiobooks", "Long-form listening", "/?view=audiobooks", "audiobook", countLabel(counts.audiobooks)}, {"Books", "Books and comics", "/?view=books", "book", countLabel(counts.books)}}},
		{"Organize", []Destination{{"My List", "Saved for later", "/?view=list", "star", countLabel(listCount)}, {"Collections", "Curated worlds", "/?view=collections", "collection", countLabel(collectionCount)}, {"Playlists", "Queues and smart mixes", "/?view=playlists", "playlist", countLabel(playlistCount)}}},
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

func countLabel(count int) string {
	return catalog.CountLabel(count)
}
