package viewing

import (
	"sort"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

const maximumPreviewRows = 500

// PreviewInput contains normalized source activity and read-only destination state.
type PreviewInput struct {
	Overwrite     bool
	Activities    []Activity
	Items         []library.Item
	Progress      func(string) catalog.PlaybackState
	ListAdditions func(string, bool, map[string]int) int
}

// BuildPreview classifies every source activity before any destination write.
func BuildPreview(input PreviewInput) Plan {
	preview := Plan{}
	preview.Summary.SourceItems = len(input.Activities)
	for _, activity := range input.Activities {
		hasProgress := activity.Watched || activity.Seconds > 0
		if !hasProgress && !activity.Favorite && len(activity.Playlists) == 0 {
			continue
		}
		countPreviewSource(&preview.Summary, activity, hasProgress)
		item := buildPreviewItem(input, activity, hasProgress, &preview)
		appendPreviewItem(&preview, item)
	}
	return preview
}

func countPreviewSource(summary *Summary, activity Activity, hasProgress bool) {
	if hasProgress {
		summary.Activity++
	}
	if activity.Favorite {
		summary.Favorites++
	}
	summary.PlaylistItems += len(activity.Playlists)
}

func buildPreviewItem(input PreviewInput, activity Activity, hasProgress bool, preview *Plan) Item {
	playlistNames := make([]string, 0, len(activity.Playlists))
	for name := range activity.Playlists {
		playlistNames = append(playlistNames, name)
	}
	sort.Strings(playlistNames)
	item := Item{SourceTitle: Label(activity), Seconds: activity.Seconds, Watched: activity.Watched, Favorite: activity.Favorite, Playlists: playlistNames}
	target, status, reason := Match(activity, input.Items)
	item.Status, item.Reason = status, reason
	if status == "ambiguous" {
		preview.Summary.Ambiguous++
		return item
	}
	if status == "unmatched" {
		preview.Summary.Unmatched++
		return item
	}
	item.TargetID, item.TargetTitle = target.ID, target.Title
	preview.Summary.Matched++
	classifyMatchedPreview(input, activity, target.ID, hasProgress, preview, &item)
	return item
}

func classifyMatchedPreview(input PreviewInput, activity Activity, targetID string, hasProgress bool, preview *Plan, item *Item) {
	progressReady, conflict := previewProgressChange(input, activity, targetID, hasProgress, preview)
	listsReady := input.ListAdditions(targetID, activity.Favorite, activity.Playlists)
	if listsReady > 0 {
		preview.ListChanges = append(preview.ListChanges, ListChange{TargetID: targetID, Favorite: activity.Favorite, Playlists: activity.Playlists})
	}
	switch {
	case progressReady || listsReady > 0:
		item.Status = "matched"
		preview.Summary.Importable++
	case conflict:
		item.Status, item.Reason = "conflict", "existing Kinosail viewing activity is preserved"
	default:
		item.Status, item.Reason = "unchanged", "Kinosail already has this state"
		preview.Summary.Unchanged++
	}
}

func previewProgressChange(input PreviewInput, activity Activity, targetID string, hasProgress bool, preview *Plan) (bool, bool) {
	if !hasProgress {
		return false, false
	}
	destination := input.Progress(targetID)
	state := catalog.PlaybackState{Seconds: activity.Seconds, Watched: activity.Watched, Updated: activity.Updated}
	if SameState(destination, state) {
		return false, false
	}
	if !input.Overwrite && destination != (catalog.PlaybackState{}) {
		preview.Summary.Conflicts++
		return false, true
	}
	preview.Changes = append(preview.Changes, ProgressChange{targetID, state, destination})
	return true, false
}

func appendPreviewItem(preview *Plan, item Item) {
	if len(preview.Items) < maximumPreviewRows {
		preview.Items = append(preview.Items, item)
		return
	}
	preview.Truncated = true
}

// NewPreview builds one expiring credential-safe preview snapshot.
func NewPreview(id string, expires time.Time, input Input, profileID, profileName string, activities []Activity, items []library.Item, progress func(string) catalog.PlaybackState, listAdditions func(string, bool, map[string]int) int) Preview {
	plan := BuildPreview(PreviewInput{Overwrite: input.Overwrite, Activities: activities, Items: items, Progress: progress, ListAdditions: listAdditions})
	return Preview{ID: id, Source: input.Source, ProfileID: profileID, ProfileName: profileName, ExpiresAt: expires, Summary: plan.Summary, Items: plan.Items, Truncated: plan.Truncated, Input: input, Activities: activities, Changes: plan.Changes, ListChanges: plan.ListChanges}
}
