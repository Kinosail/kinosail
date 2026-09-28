package server

import (
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const subtitleHistoryPageSize = 20

func (manager *subtitleManager) subtitleHistoryProjection(options subtitleDashboardOptions, items []library.Item) (subtitleDashboardData, error) {
	events, err := manager.provider.ledger.history()
	if err != nil {
		return subtitleDashboardData{}, err
	}
	byID := make(map[string]library.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	data := subtitleDashboardData{subtitleDashboardOptions: options, ServerName: manager.settings.serverName(), PageSize: subtitleHistoryPageSize, Matched: len(events), History: []subtitleHistoryView{}}
	data.Pages = max(1, (data.Matched+data.PageSize-1)/data.PageSize)
	data.Page = min(data.Page, data.Pages)
	start := (data.Page - 1) * data.PageSize
	end := min(start+data.PageSize, data.Matched)
	for i := len(events) - 1 - start; i >= len(events)-end; i-- {
		event := events[i]
		parts := strings.SplitN(event.Key, ":", 2)
		changed := time.Unix(event.InstalledAt, 0).UTC()
		view := subtitleHistoryView{ID: parts[0], Action: event.Action, Title: "File no longer in library", Language: parts[1], Source: event.Source, SourceLabel: subtitleHistorySourceLabel(event.Source), Changed: changed.Format(time.RFC3339), DisplayTime: changed.Format("2006-01-02 15:04") + " UTC"}
		if item, found := byID[view.ID]; found {
			identity := subtitleDashboardIdentity(item)
			view.Title, view.Context, view.Available = identity.Title, identity.Context, true
		}
		data.History = append(data.History, view)
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

func subtitleHistorySourceLabel(source string) string {
	switch source {
	case "subdl":
		return "SubDL"
	case "opensubtitles":
		return "OpenSubtitles"
	case "subsource":
		return "SubSource"
	case "embedded":
		return "Embedded track"
	case "ocr":
		return "Local OCR"
	case "transcription":
		return "Local transcription"
	case "external":
		return "Local file"
	default:
		return "Subtitle source"
	}
}
