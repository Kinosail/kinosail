package jellyfincompat

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	MoviesID = "00000000000000000000000000000001"
	ShowsID  = "00000000000000000000000000000002"
)

type ProjectItem func(library.Item) map[string]any

type Catalog struct {
	view    *library.View
	project ProjectItem
}

func NewCatalog(view *library.View, project ProjectItem) Catalog {
	return Catalog{view: view, project: project}
}

func Result(items []map[string]any, start int, totals ...int) map[string]any {
	total := len(items)
	if len(totals) > 0 {
		total = totals[0]
	}
	return map[string]any{"Items": items, "StartIndex": start, "TotalRecordCount": total}
}

func Query(values url.Values, name string) string {
	for key, candidates := range values {
		if strings.EqualFold(key, name) && len(candidates) > 0 {
			return candidates[0]
		}
	}
	return ""
}

func Int(values url.Values, name string) int {
	value, _ := strconv.Atoi(Query(values, name))
	return value
}

func Includes(values, value string) bool {
	return includesType(strings.Split(values, ","), value)
}

func includesType(candidates []string, value string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}

func (catalog Catalog) Views() map[string]any {
	views := make([]map[string]any, 0, 2)
	if len(catalog.view.Movies()) > 0 {
		views = append(views, FolderDTO(MoviesID, "Movies", "movies"))
	}
	if len(catalog.view.Shows()) > 0 {
		views = append(views, FolderDTO(ShowsID, "Shows", "tvshows"))
	}
	return Result(views, 0)
}

func (catalog Catalog) Items(values url.Values) (map[string]any, error) {
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return nil, err
	}
	result := catalog.query(values, ids)
	total := len(result)
	start, limit := max(0, Int(values, "startIndex")), Int(values, "limit")
	if start > len(result) {
		start = len(result)
	}
	result = result[start:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return Result(catalog.projectEntries(result), start, total), nil
}

func (catalog Catalog) Latest(values url.Values) ([]map[string]any, error) {
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return nil, err
	}
	result := catalog.query(values, ids)
	if result != nil {
		result = slices.Clone(result)
		created := make(map[*library.Item]string, len(result))
		for _, entry := range result {
			if entry.item != nil {
				created[entry.item] = entry.created()
			}
		}
		slices.SortStableFunc(result, func(left, right catalogEntry) int {
			return strings.Compare(created[right.item], created[left.item])
		})
	}
	if limit := Int(values, "limit"); limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return catalog.projectEntries(result), nil
}

func (catalog Catalog) Resume(state func(library.Item) UserState) map[string]any {
	result := make([]map[string]any, 0)
	for _, item := range catalog.view.Items() {
		if item.Kind != "video" {
			continue
		}
		if progress := state(item); progress.Seconds > 0 && !progress.Watched {
			result = append(result, catalog.project(item))
		}
	}
	return Result(result, 0)
}

func (catalog Catalog) NextUp(watched func(library.Item) bool) map[string]any {
	shows := catalog.view.Shows()
	result := make([]map[string]any, 0, len(shows))
	for _, show := range shows {
		for _, episode := range show.Episodes {
			if !watched(episode) {
				result = append(result, catalog.project(episode))
				break
			}
		}
	}
	return Result(result, 0)
}

func (catalog Catalog) Item(id string) (map[string]any, bool) {
	if item, found := catalog.view.Find(RawID(id)); found {
		return catalog.project(item), true
	}
	if show, found := catalog.view.Show(RawID(id)); found && (ID(show.ID) == id || show.ID == id) {
		return SeriesDTO(show), true
	}
	if show, found := catalog.seasonShow(id); found {
		for _, season := range Seasons(show) {
			if SeasonID(show.ID, season) == id {
				return SeasonDTO(show, season), true
			}
		}
	}
	return nil, false
}

func (catalog Catalog) SeasonsResult(id string) (map[string]any, bool) {
	show, found := catalog.view.Show(RawID(id))
	if !found {
		return nil, false
	}
	result := make([]map[string]any, 0)
	for _, season := range Seasons(show) {
		result = append(result, SeasonDTO(show, season))
	}
	return Result(result, 0), true
}

func (catalog Catalog) Episodes(id string, values url.Values) (map[string]any, bool) {
	show, found := catalog.view.Show(RawID(id))
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

func (catalog Catalog) seasonShow(id string) (library.Show, bool) {
	if len(id) < 16 {
		return library.Show{}, false
	}
	return catalog.view.Show(id[:16])
}

func (catalog Catalog) query(values url.Values, ids map[string]bool) []catalogEntry {
	parent, includes := Query(values, "parentId"), Query(values, "includeItemTypes")
	result := catalog.under(parent, includes)
	search := strings.ToLower(Query(values, "searchTerm"))
	if result == nil || len(ids) == 0 && includes == "" && search == "" {
		return result
	}
	types := strings.Split(includes, ",")
	filtered := make([]catalogEntry, 0)
	for _, entry := range result {
		matchesID := len(ids) == 0 || ids[entry.raw]
		matchesType := includes == "" || includesType(types, entry.kind)
		matchesName := search == "" || strings.Contains(entry.lower, search)
		if matchesID && matchesType && matchesName {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (catalog Catalog) under(parent, includes string) []catalogEntry {
	switch parent {
	case MoviesID:
		return library.Derive(catalog.view, "jellyfincompat.movies", movieEntries)
	case ShowsID:
		if Includes(includes, "Episode") {
			return library.Derive(catalog.view, "jellyfincompat.episodes", episodeEntries)
		}
		return library.Derive(catalog.view, "jellyfincompat.series", seriesEntries)
	case "":
		if Includes(includes, "Episode") {
			return library.Derive(catalog.view, "jellyfincompat.episodes", episodeEntries)
		}
		return library.Derive(catalog.view, "jellyfincompat.root", rootEntries)
	default:
		return catalog.hierarchy(parent)
	}
}

func (catalog Catalog) hierarchy(parent string) []catalogEntry {
	show, found := catalog.seasonShow(parent)
	if !found {
		return nil
	}
	if parent == ID(show.ID) {
		result := make([]catalogEntry, 0)
		for _, season := range Seasons(show) {
			result = append(result, seasonEntry(&show, season))
		}
		return result
	}
	result := make([]catalogEntry, 0)
	for position := range show.Episodes {
		if parent == SeasonID(show.ID, show.Episodes[position].Season) {
			result = append(result, itemEntry(&show.Episodes[position]))
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (catalog Catalog) projectEntries(entries []catalogEntry) []map[string]any {
	if entries == nil {
		return nil
	}
	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		switch {
		case entry.item != nil:
			result = append(result, catalog.project(*entry.item))
		case entry.kind == "Season":
			result = append(result, SeasonDTO(*entry.show, entry.season))
		default:
			result = append(result, SeriesDTO(*entry.show))
		}
	}
	return result
}
