package jellyfincompat

import (
	"net/url"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type ItemState struct {
	Seconds  float64
	Watched  bool
	Favorite bool
}

type stateQuery struct {
	played, unplayed, favorite, unfavorite, resumable bool
	lookup                                            func(library.Item) ItemState
}

func (catalog Catalog) WithState(lookup func(library.Item) ItemState) Catalog {
	catalog.state = lookup
	return catalog
}

func parseStateQuery(values url.Values) (stateQuery, error) {
	var query stateQuery
	filters := Query(values, "filters")
	if len(filters) > maximumSortText {
		return stateQuery{}, errInvalidListQuery
	}
	for name := range strings.SplitSeq(strings.ToLower(filters), ",") {
		switch strings.TrimSpace(name) {
		case "isplayed":
			query.played = true
		case "isunplayed":
			query.unplayed = true
		case "isfavorite":
			query.favorite = true
		case "isresumable":
			query.resumable = true
		}
	}
	for name, targets := range map[string][2]*bool{"isPlayed": {&query.played, &query.unplayed}, "isFavorite": {&query.favorite, &query.unfavorite}} {
		switch strings.ToLower(Query(values, name)) {
		case "":
		case "true":
			*targets[0] = true
		case "false":
			*targets[1] = true
		default:
			return stateQuery{}, errInvalidListQuery
		}
	}
	return query, nil
}

func (query stateQuery) filters() bool {
	return query.played || query.unplayed || query.favorite || query.unfavorite || query.resumable
}

func (query stateQuery) matches(entry catalogEntry) bool {
	if !query.filters() {
		return true
	}
	state := query.state(entry)
	return (!query.played || state.Watched) && (!query.unplayed || !state.Watched) &&
		(!query.favorite || state.Favorite) && (!query.unfavorite || !state.Favorite) &&
		(!query.resumable || (state.Seconds > 0 && !state.Watched))
}

func (query stateQuery) state(entry catalogEntry) ItemState {
	if query.lookup == nil {
		return ItemState{}
	}
	if entry.item != nil {
		return query.lookup(*entry.item)
	}
	if entry.show == nil {
		return ItemState{}
	}
	episodes := 0
	for position := range entry.show.Episodes {
		episode := entry.show.Episodes[position]
		if entry.kind == "Season" && episode.Season != entry.season {
			continue
		}
		if !query.lookup(episode).Watched {
			return ItemState{}
		}
		episodes++
	}
	return ItemState{Watched: episodes > 0}
}
