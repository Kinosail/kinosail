package jellyfincompat

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	// MoviesID is the stable Jellyfin collection identifier for movies.
	MoviesID = "00000000000000000000000000000001"
	// ShowsID is the stable Jellyfin collection identifier for series.
	ShowsID = "00000000000000000000000000000002"
)

// ProjectItem adds app-owned state and media details to one shared item DTO.
type ProjectItem func(library.Item) map[string]any

// Catalog owns Jellyfin library projection, filtering, hierarchy, and feeds.
type Catalog struct {
	items   []library.Item
	movies  []library.Item
	shows   []library.Show
	project ProjectItem
}

// NewCatalog organizes one visible library behind the Jellyfin catalog interface.
func NewCatalog(items []library.Item, project ProjectItem) Catalog {
	movies, shows := library.Organize(items)
	return Catalog{items: items, movies: movies, shows: shows, project: project}
}

// Result returns one Jellyfin paging envelope.
func Result(items []map[string]any, start int, totals ...int) map[string]any {
	total := len(items)
	if len(totals) > 0 {
		total = totals[0]
	}
	return map[string]any{"Items": items, "StartIndex": start, "TotalRecordCount": total}
}

// Query returns the first case-insensitive query value.
func Query(values url.Values, name string) string {
	for key, candidates := range values {
		if strings.EqualFold(key, name) && len(candidates) > 0 {
			return candidates[0]
		}
	}
	return ""
}

// Int returns one case-insensitive integer query value or zero.
func Int(values url.Values, name string) int {
	value, _ := strconv.Atoi(Query(values, name))
	return value
}

// Includes reports whether a comma-separated value contains one case-insensitive item.
func Includes(values, value string) bool {
	for _, candidate := range strings.Split(values, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}

// Views returns visible movie and series folders.
func (catalog Catalog) Views() map[string]any {
	views := make([]map[string]any, 0, 2)
	if len(catalog.movies) > 0 {
		views = append(views, FolderDTO(MoviesID, "Movies", "movies"))
	}
	if len(catalog.shows) > 0 {
		views = append(views, FolderDTO(ShowsID, "Shows", "tvshows"))
	}
	return Result(views, 0)
}

// Items filters and pages the visible Jellyfin catalog.
func (catalog Catalog) Items(values url.Values) (map[string]any, error) {
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return nil, err
	}
	result := catalog.query(values, ids)
	total := len(result)
	start, limit := Int(values, "startIndex"), Int(values, "limit")
	if start > len(result) {
		start = len(result)
	}
	result = result[start:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return Result(result, start, total), nil
}

// Latest returns visible items ordered by descending creation time.
func (catalog Catalog) Latest(values url.Values) ([]map[string]any, error) {
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return nil, err
	}
	result := catalog.query(values, ids)
	sort.SliceStable(result, func(left, right int) bool {
		return stringValue(result[left]["DateCreated"]) > stringValue(result[right]["DateCreated"])
	})
	if limit := Int(values, "limit"); limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

// Resume returns visible videos with unfinished playback.
func (catalog Catalog) Resume(state func(library.Item) UserState) map[string]any {
	result := make([]map[string]any, 0)
	for _, item := range catalog.items {
		progress := state(item)
		if item.Kind == "video" && progress.Seconds > 0 && !progress.Watched {
			result = append(result, catalog.project(item))
		}
	}
	return Result(result, 0)
}

// NextUp returns the first unwatched episode from each visible series.
func (catalog Catalog) NextUp(watched func(library.Item) bool) map[string]any {
	result := make([]map[string]any, 0, len(catalog.shows))
	for _, show := range catalog.shows {
		for _, episode := range show.Episodes {
			if !watched(episode) {
				result = append(result, catalog.project(episode))
				break
			}
		}
	}
	return Result(result, 0)
}

// Item returns one visible item, series, or season DTO.
func (catalog Catalog) Item(id string) (map[string]any, bool) {
	if item, found := findItem(catalog.items, RawID(id)); found {
		return catalog.project(item), true
	}
	for _, show := range catalog.shows {
		if ID(show.ID) == id || show.ID == id {
			return SeriesDTO(show), true
		}
		for _, season := range Seasons(show) {
			if SeasonID(show.ID, season) == id {
				return SeasonDTO(show, season), true
			}
		}
	}
	return nil, false
}

// SeasonsResult returns the visible seasons for one series.
func (catalog Catalog) SeasonsResult(id string) (map[string]any, bool) {
	show, found := catalog.show(RawID(id))
	if !found {
		return nil, false
	}
	result := make([]map[string]any, 0)
	for _, season := range Seasons(show) {
		result = append(result, SeasonDTO(show, season))
	}
	return Result(result, 0), true
}

// Episodes returns visible episodes, optionally filtered by season.
func (catalog Catalog) Episodes(id string, values url.Values) (map[string]any, bool) {
	show, found := catalog.show(RawID(id))
	if !found {
		return nil, false
	}
	seasonID, seasonNumber := Query(values, "seasonId"), Int(values, "season")
	result := make([]map[string]any, 0, len(show.Episodes))
	for _, item := range show.Episodes {
		all := seasonID == "" && seasonNumber == 0
		if all || seasonID == SeasonID(show.ID, item.Season) || seasonNumber == item.Season {
			result = append(result, catalog.project(item))
		}
	}
	return Result(result, 0), true
}

func (catalog Catalog) query(values url.Values, ids map[string]bool) []map[string]any {
	parent, includes := Query(values, "parentId"), Query(values, "includeItemTypes")
	result := catalog.under(parent, includes)
	search := strings.ToLower(Query(values, "searchTerm"))
	filtered := result[:0]
	for _, item := range result {
		kind, name, id := stringValue(item["Type"]), stringValue(item["Name"]), stringValue(item["Id"])
		matchesID := len(ids) == 0 || ids[RawID(id)]
		matchesType := includes == "" || Includes(includes, kind)
		matchesName := search == "" || strings.Contains(strings.ToLower(name), search)
		if matchesID && matchesType && matchesName {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (catalog Catalog) under(parent, includes string) []map[string]any {
	switch parent {
	case MoviesID:
		return catalog.itemDTOs(catalog.movies)
	case ShowsID:
		if Includes(includes, "Episode") {
			return catalog.itemDTOs(allEpisodes(catalog.shows))
		}
		return seriesDTOs(catalog.shows)
	case "":
		if Includes(includes, "Episode") {
			return catalog.itemDTOs(allEpisodes(catalog.shows))
		}
		return catalog.rootDTOs()
	default:
		return catalog.hierarchyDTOs(parent)
	}
}

func (catalog Catalog) rootDTOs() []map[string]any {
	result := append(catalog.itemDTOs(catalog.movies), seriesDTOs(catalog.shows)...)
	for _, item := range catalog.items {
		if item.Kind != "video" {
			result = append(result, catalog.project(item))
		}
	}
	return result
}

func (catalog Catalog) hierarchyDTOs(parent string) []map[string]any {
	for _, show := range catalog.shows {
		if parent == ID(show.ID) {
			result := make([]map[string]any, 0)
			for _, season := range Seasons(show) {
				result = append(result, SeasonDTO(show, season))
			}
			return result
		}
		episodes := make([]library.Item, 0)
		for _, item := range show.Episodes {
			if parent == SeasonID(show.ID, item.Season) {
				episodes = append(episodes, item)
			}
		}
		if len(episodes) > 0 {
			return catalog.itemDTOs(episodes)
		}
	}
	return nil
}

func (catalog Catalog) itemDTOs(items []library.Item) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, catalog.project(item))
	}
	return result
}

func (catalog Catalog) show(id string) (library.Show, bool) {
	for _, show := range catalog.shows {
		if show.ID == id {
			return show, true
		}
	}
	return library.Show{}, false
}

func allEpisodes(shows []library.Show) []library.Item {
	result := make([]library.Item, 0)
	for _, show := range shows {
		result = append(result, show.Episodes...)
	}
	return result
}

func findItem(items []library.Item, id string) (library.Item, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return library.Item{}, false
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
