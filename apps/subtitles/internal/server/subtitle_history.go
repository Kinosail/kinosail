package server

import (
	"fmt"
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
		view.subtitleHistoryEvidence = event.subtitleHistoryEvidence
		view.ActionLabel, view.Explanation, view.Match = subtitleHistoryDescription(event)
		if view.ActionLabel == "Recorded" {
			view.Action = "recorded"
		}
		if event.PreviousSource != "" {
			view.SourceLabel = subtitleHistorySourceLabel(event.PreviousSource) + " → " + view.SourceLabel
		}
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

func subtitleHistoryDescription(event subtitleHistoryEvent) (string, string, string) { //nolint:cyclop // Each recorded operation has one factual explanation.
	action, explanation := "Recorded", "The reason for this earlier change was not recorded."
	switch event.Action {
	case "added":
		action = "Added"
	case "updated":
		action = "Updated"
	case "restored":
		action = "Restored"
	}
	switch event.Reason {
	case "missing":
		explanation = "Added a new subtitle file for a missing language."
	case "embedded":
		explanation = "Copied an embedded text track to a new subtitle file."
	case "higher-score":
		action, explanation = "Upgraded", fmt.Sprintf("Automatic upgrade: match score improved by %d points.", *event.Score-*event.PreviousScore)
	case "exact-hash":
		action, explanation = "Upgraded", "Automatic upgrade: exact file-hash match. The previous local file had no verified match score."
	case "manual":
		explanation = "Saved manually in the subtitle editor."
	case "restore":
		explanation = "Restored the previous subtitle from its recovery copy."
	case "legacy":
		explanation = "Earlier installation; whether it added or replaced a file was not recorded."
	}
	if event.Reason == "" && event.Action == "added" {
		action = "Recorded"
	}
	match := ""
	if event.Score != nil {
		match = fmt.Sprintf("Match score %d / 100 · Release match %.0f%%", *event.Score, *event.ReleaseMatch*100)
		if event.PreviousScore != nil {
			match = fmt.Sprintf("Match score %d → %d / 100 · Release match %.0f%% → %.0f%%", *event.PreviousScore, *event.Score, *event.PreviousReleaseMatch*100, *event.ReleaseMatch*100)
		}
	}
	return action, explanation, match
}
