package library

import "sync"

type View struct {
	items     []Item
	byID      map[string]int
	video     sync.Once
	movies    []Item
	shows     []Show
	showAt    map[string]int
	next      map[string]string
	music     sync.Once
	albums    []Album
	albumAt   map[string]int
	trackAt   map[string][2]int
	derivedMu sync.Mutex
	derived   map[string]*derivedValue
}

type derivedValue struct {
	once  sync.Once
	value any
}

func newView(items []Item, byID map[string]int) *View {
	return &View{items: items, byID: byID, derived: make(map[string]*derivedValue)}
}

func (view *View) Items() []Item { return view.items }

func (view *View) Find(id string) (Item, bool) {
	position, found := view.byID[id]
	if !found {
		return Item{}, false
	}
	return view.items[position], true
}

func (view *View) organizeVideo() {
	view.video.Do(func() {
		view.movies, view.shows = Organize(view.items)
		view.showAt, view.next = make(map[string]int, len(view.shows)), make(map[string]string)
		for position, show := range view.shows {
			view.showAt[show.ID] = position
			for episode := 1; episode < len(show.Episodes); episode++ {
				if _, found := view.next[show.Episodes[episode-1].ID]; !found {
					view.next[show.Episodes[episode-1].ID] = show.Episodes[episode].ID
				}
			}
		}
	})
}

func (view *View) Movies() []Item {
	view.organizeVideo()
	return view.movies
}

func (view *View) Shows() []Show {
	view.organizeVideo()
	return view.shows
}

func (view *View) Show(id string) (Show, bool) {
	view.organizeVideo()
	position, found := view.showAt[id]
	if !found {
		return Show{}, false
	}
	return view.shows[position], true
}

func (view *View) NextEpisode(id string) string {
	view.organizeVideo()
	return view.next[id]
}

func (view *View) organizeMusic() {
	view.music.Do(func() {
		_, view.albums = OrganizeMusic(view.items)
		view.albumAt, view.trackAt = make(map[string]int, len(view.albums)), make(map[string][2]int)
		for album, value := range view.albums {
			view.albumAt[value.ID] = album
			for track, item := range value.Tracks {
				if _, found := view.trackAt[item.ID]; !found {
					view.trackAt[item.ID] = [2]int{album, track}
				}
			}
		}
	})
}

func (view *View) Albums() []Album {
	view.organizeMusic()
	return view.albums
}

func (view *View) Album(id string) (Album, bool) {
	view.organizeMusic()
	position, found := view.albumAt[id]
	if !found {
		return Album{}, false
	}
	return view.albums[position], true
}

func (view *View) AudioQueue(id string) ([]Item, bool) {
	view.organizeMusic()
	position, found := view.trackAt[id]
	if !found {
		return nil, false
	}
	return view.albums[position[0]].Tracks[position[1]:], true
}

func (view *View) Collections() []string {
	return Derive(view, "collections", func(view *View) []string {
		names := make([]string, 0)
		seen := make(map[string]bool)
		for position := range view.items {
			if name := view.items[position].Collection; name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		return names
	})
}

func Derive[T any](view *View, key string, build func(*View) T) T {
	view.derivedMu.Lock()
	entry := view.derived[key]
	if entry == nil {
		entry = &derivedValue{}
		view.derived[key] = entry
	}
	view.derivedMu.Unlock()
	entry.once.Do(func() { entry.value = build(view) })
	return entry.value.(T)
}
