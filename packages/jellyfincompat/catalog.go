package jellyfincompat

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	MoviesID = "00000000000000000000000000000001"
	ShowsID  = "00000000000000000000000000000002"
)

type ProjectItem func(library.Item) ItemRecord

type Catalog struct {
	view    *library.View
	project ProjectItem
}

func NewCatalog(view *library.View, project ProjectItem) Catalog {
	return Catalog{view: view, project: project}
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

func (catalog Catalog) Views() Page {
	views := make([]FolderRecord, 0, 2)
	if len(catalog.view.Movies()) > 0 {
		views = append(views, FolderDTO(MoviesID, "Movies", "movies"))
	}
	if len(catalog.view.Shows()) > 0 {
		views = append(views, FolderDTO(ShowsID, "Shows", "tvshows"))
	}
	return Result(views, 0)
}

func (catalog Catalog) Items(values url.Values) (Page, error) {
	limit, err := itemsLimit(values)
	if err != nil {
		return Page{}, err
	}
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return Page{}, err
	}
	entries := catalog.under(Query(values, "parentId"), Query(values, "includeItemTypes"))
	if entries == nil {
		return Page{Items: catalog.records(nil)}, nil
	}
	page, start, total := newEntryFilter(values, ids).page(entries, max(0, Int(values, "startIndex")), limit)
	return Page{Items: catalog.records(page), StartIndex: start, TotalRecordCount: total}, nil
}

func (catalog Catalog) Latest(values url.Values) (Records, error) {
	limit, err := latestLimit(values)
	if err != nil {
		return Records{}, err
	}
	ids, err := ItemIDs(values, RawID)
	if err != nil {
		return Records{}, err
	}
	entries := catalog.newest(Query(values, "parentId"), Query(values, "includeItemTypes"))
	if entries == nil {
		return catalog.records(nil), nil
	}
	return catalog.records(newEntryFilter(values, ids).first(entries, limit)), nil
}

func (catalog Catalog) Resume(state func(library.Item) UserState) Page {
	items := catalog.view.Items()
	result := make([]catalogEntry, 0)
	for position := range items {
		if items[position].Kind != "video" {
			continue
		}
		if progress := state(items[position]); progress.Seconds > 0 && !progress.Watched {
			result = append(result, catalogEntry{item: &items[position]})
		}
	}
	return catalog.page(result)
}

func (catalog Catalog) NextUp(watched func(library.Item) bool) Page {
	shows := catalog.view.Shows()
	result := make([]catalogEntry, 0, len(shows))
	for _, show := range shows {
		for position := range show.Episodes {
			if !watched(show.Episodes[position]) {
				result = append(result, catalogEntry{item: &show.Episodes[position]})
				break
			}
		}
	}
	return catalog.page(result)
}

func (catalog Catalog) Item(id string) (any, bool) {
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

func (catalog Catalog) SeasonsResult(id string) (Page, bool) {
	show, found := catalog.view.Show(RawID(id))
	if !found {
		return Page{}, false
	}
	result := make([]catalogEntry, 0)
	for _, season := range Seasons(show) {
		result = append(result, seasonEntry(&show, season))
	}
	return catalog.page(result), true
}

func (catalog Catalog) Episodes(id string, values url.Values) (Page, bool) {
	show, found := catalog.view.Show(RawID(id))
	if !found {
		return Page{}, false
	}
	seasonID, seasonNumber := Query(values, "seasonId"), Int(values, "season")
	result := make([]catalogEntry, 0, len(show.Episodes))
	for position := range show.Episodes {
		item := &show.Episodes[position]
		all := seasonID == "" && seasonNumber == 0
		if all || seasonID == SeasonID(show.ID, item.Season) || seasonNumber == item.Season {
			result = append(result, catalogEntry{item: item})
		}
	}
	return catalog.page(result), true
}

func (catalog Catalog) records(entries []catalogEntry) Records {
	return Records{entries: entries, project: catalog.project}
}

func (catalog Catalog) page(entries []catalogEntry) Page {
	return Page{Items: catalog.records(entries), TotalRecordCount: len(entries)}
}

func (catalog Catalog) seasonShow(id string) (library.Show, bool) {
	if len(id) < 16 {
		return library.Show{}, false
	}
	return catalog.view.Show(id[:16])
}

func (catalog Catalog) under(parent, includes string) []catalogEntry {
	if key, build := catalogListing(parent, includes); build != nil {
		return library.Derive(catalog.view, key, build)
	}
	return catalog.hierarchy(parent)
}

func (catalog Catalog) newest(parent, includes string) []catalogEntry {
	key, build := catalogListing(parent, includes)
	if build == nil {
		return newestFirst(catalog.hierarchy(parent))
	}
	return library.Derive(catalog.view, key+".newest", func(view *library.View) []catalogEntry {
		return newestFirst(library.Derive(view, key, build))
	})
}

func catalogListing(parent, includes string) (string, func(*library.View) []catalogEntry) {
	episodes := Includes(includes, "Episode")
	switch {
	case parent == MoviesID:
		return "jellyfincompat.movies", movieEntries
	case (parent == ShowsID || parent == "") && episodes:
		return "jellyfincompat.episodes", episodeEntries
	case parent == ShowsID:
		return "jellyfincompat.series", seriesEntries
	case parent == "":
		return "jellyfincompat.root", rootEntries
	default:
		return "", nil
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
