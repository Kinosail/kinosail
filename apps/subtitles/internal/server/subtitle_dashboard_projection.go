package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleDashboardCandidate struct {
	source    library.Item
	row       subtitleDashboardItem
	sortTitle string
}

func (manager *subtitleManager) projection(request *http.Request, options subtitleDashboardOptions) (subtitleDashboardData, error) {
	items, err := visibleLibrary(request, manager.index)
	if err != nil {
		return subtitleDashboardData{}, err
	}
	languages := manager.settings.subtitleLanguages()
	data := subtitleDashboardData{subtitleDashboardOptions: options, ServerName: manager.settings.serverName(), Language: languages[0], Languages: languages, LanguageSummary: strings.Join(languages, ", "), ProviderConfigured: manager.provider.configured(), Preference: manager.settings.subtitlePreference(), PageSize: subtitleLibraryPageSize, Items: []subtitleDashboardItem{}}
	for _, language := range languages {
		data.ProviderReady = data.ProviderReady || manager.provider.supports(language)
		data.ProviderAvailable = data.ProviderAvailable || subtitleProviderAvailable(language)
	}
	// Browsing a library never performs readiness write probes on its media roots.
	if options.View == "summary" {
		data.PageSize = 6
		data.Readiness, data.Providers = manager.readiness(), manager.provider.healthViews()
		data.ProviderReady = data.ProviderReady || data.Readiness.State == "Ready"
	}
	candidates := make([]subtitleDashboardCandidate, 0)
	for _, item := range items {
		if err := request.Context().Err(); err != nil {
			return subtitleDashboardData{}, err
		}
		if item.Kind != "video" {
			continue
		}
		row := manager.subtitleDashboardCoverage(item, languages)
		data.Total++
		switch row.State {
		case "checking":
			data.Pending++
		case "unavailable":
			data.Unavailable++
		case "ready":
			data.Ready++
		default:
			data.Wanted++
		}
		if !options.includes(item, row) {
			continue
		}
		key := strings.ToLower(row.Title)
		candidates = append(candidates, subtitleDashboardCandidate{source: item, row: row, sortTitle: key})
	}
	if data.Total > 0 {
		data.Coverage = data.Ready * 100 / data.Total
	}
	sort.Slice(candidates, func(left, right int) bool {
		a, b := candidates[left], candidates[right]
		if options.Sort == "modified" && !a.source.Added.Equal(b.source.Added) {
			return a.source.Added.After(b.source.Added)
		}
		if a.sortTitle != b.sortTitle {
			return a.sortTitle < b.sortTitle
		}
		if a.row.MediaKind == "episode" && b.row.MediaKind == "episode" {
			if a.source.Season != b.source.Season {
				return a.source.Season < b.source.Season
			}
			if a.source.Episode != b.source.Episode {
				return a.source.Episode < b.source.Episode
			}
		}
		return a.source.ID < b.source.ID
	})
	data.Matched = len(candidates)
	data.Pages = max(1, (data.Matched+data.PageSize-1)/data.PageSize)
	data.Page = min(data.Page, data.Pages)
	start := (data.Page - 1) * data.PageSize
	end := min(start+data.PageSize, data.Matched)
	// Only visible rows get history, sidecar actions, and recovery-file checks.
	for _, candidate := range candidates[start:end] {
		row := candidate.row
		manager.decorateSubtitleDashboardItem(&row, candidate.source, languages)
		data.Items = append(data.Items, row)
	}
	if data.Matched > 0 {
		data.Start, data.End = start+1, end
	}
	if data.Page > 1 {
		data.PreviousURL = data.pageURL(data.Page - 1)
	}
	if data.Page < data.Pages && options.View != "summary" {
		data.NextURL = data.pageURL(data.Page + 1)
	}
	data.FilterURL = data.pageURL(1)
	return data, nil
}

func (options subtitleDashboardOptions) includes(item library.Item, row subtitleDashboardItem) bool {
	if options.View != "library" && row.State != "wanted" {
		return false
	}
	if options.Status != "all" && options.Status != row.State {
		return false
	}
	if options.Kind != "all" && options.Kind != row.MediaKind {
		return false
	}
	return options.Query == "" || subtitleMatches(item, options.Query)
}

func (manager *subtitleManager) subtitleDashboardCoverage(item library.Item, languages []string) subtitleDashboardItem {
	row := subtitleDashboardIdentity(item)
	media, checked, failed := manager.cachedFacts(item)
	if !checked {
		row.Pending, row.Unavailable, row.State = !failed, failed, "checking"
		if failed {
			row.State = "unavailable"
		}
		return row
	}
	ready, tracks, missing := manager.planCoverageAllFacts(item, languages, media)
	row.Ready, row.Tracks, row.Missing, row.State = ready, strings.Join(tracks, ", "), missing, "wanted"
	if ready {
		row.State = "ready"
	}

	return row
}

func (manager *subtitleManager) decorateSubtitleDashboardItem(row *subtitleDashboardItem, item library.Item, languages []string) {
	if row.Pending || row.Unavailable {
		return
	}
	media, _, _ := manager.cachedFacts(item)
	row.ActionLanguage, row.Searchable = manager.searchableSubtitleFacts(row.Missing, media)
	if language := manager.subtitleSidecarLanguage(item, languages, strings.Split(row.Tracks, ", "), row.Ready); language != "" {
		row.NeedsSidecar, row.ActionLanguage = true, language
	}
	language := languages[0]

	row.WantedReason, row.Restorable = "Preferred subtitle is missing", subtitleBackupAvailable(item, language)
	manager.addSubtitleDashboardRecord(row, item, language, row.Ready)
	if row.ActionLanguage != "" {
		language = row.ActionLanguage
	}
	manager.addSubtitleDashboardSearch(row, item, language, row.Ready)
}
