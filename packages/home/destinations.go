package home

type Destination struct {
	Name, Description, Href, Icon string
	Count                         int
}

type DestinationGroup struct {
	Name         string
	Destinations []Destination
}

func destinations(counts libraryCounts, listCount, collectionCount, playlistCount int) []DestinationGroup {
	groups := []DestinationGroup{
		{"Watch & view", []Destination{{"Movies", "Feature films", "/?view=movies", "play", counts.movies}, {"Shows", "Series and episodes", "/?view=shows", "shows", counts.shows}, {"Photos", "Photo library", "/?view=photos", "photo", counts.photos}}},
		{"Listen & read", []Destination{{"Music", "Albums and tracks", "/?view=music", "music", counts.music}, {"Audiobooks", "Long-form listening", "/?view=audiobooks", "audiobook", counts.audiobooks}, {"Books", "Books and comics", "/?view=books", "book", counts.books}}},
		{"Organize", []Destination{{"My List", "Saved for later", "/?view=list", "star", listCount}, {"Collections", "Curated worlds", "/?view=collections", "collection", collectionCount}, {"Playlists", "Queues and smart mixes", "/?view=playlists", "playlist", playlistCount}}},
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
		if destination.Count != 0 {
			visible = append(visible, destination)
		}
	}
	return visible
}
