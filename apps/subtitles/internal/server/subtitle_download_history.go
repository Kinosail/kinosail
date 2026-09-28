package server

import (
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const subtitleDownloadPageSize = 20

func (manager *subtitleManager) downloadHistoryProjection(options subtitleDashboardOptions, items []library.Item) (subtitleDashboardData, error) {
	events, err := manager.provider.ledger.downloads()
	if err != nil {
		return subtitleDashboardData{}, err
	}
	byID := make(map[string]library.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	data := subtitleDashboardData{subtitleDashboardOptions: options, ServerName: manager.settings.serverName(), PageSize: subtitleDownloadPageSize, Matched: len(events), Downloads: []subtitleDownloadView{}}
	data.Pages = max(1, (data.Matched+data.PageSize-1)/data.PageSize)
	data.Page = min(data.Page, data.Pages)
	start := (data.Page - 1) * data.PageSize
	end := min(start+data.PageSize, data.Matched)
	for i := len(events) - 1 - start; i >= len(events)-end; i-- {
		event := events[i]
		parts := strings.SplitN(event.Key, ":", 2)
		view := subtitleDownloadView{ID: parts[0], Title: "File no longer in library", Language: parts[1], Source: event.Source, SourceLabel: subtitleDownloadSourceLabel(event.Source), Installed: time.Unix(event.InstalledAt, 0).UTC().Format(time.RFC3339)}
		if item, found := byID[view.ID]; found {
			identity := subtitleDashboardIdentity(item)
			view.Title, view.Context, view.Available = identity.Title, identity.Context, true
		}
		data.Downloads = append(data.Downloads, view)
	}
	if data.Matched > 0 {
		data.Start, data.End = start+1, end
	}
	if data.Page > 1 {
		data.PreviousURL = data.pageURL(data.Page - 1)
	}
	if data.Page < data.Pages {
		data.NextURL = data.pageURL(data.Page + 1)
	}
	data.FilterURL = data.pageURL(1)
	return data, nil
}

func subtitleDownloadSourceLabel(source string) string {
	switch source {
	case "subdl":
		return "SubDL"
	case "opensubtitles":
		return "OpenSubtitles"
	case "subsource":
		return "SubSource"
	default:
		return "Subtitle source"
	}
}
