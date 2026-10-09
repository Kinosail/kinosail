package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) applySeekPlayback(request *http.Request, item library.Item, facts MediaFacts, data *playerData) error {
	position := savedSeekPosition(data.Start, data.Duration)
	if position == 0 || data.Kind != "video" || data.Plan.MarkerMode == "server" {
		return nil
	}
	source := data.Source
	if data.Plan.Mode == "direct" {
		source = data.AdaptiveSource
	}
	if source == "" {
		return nil
	} // Explicit Direct has no compatible source.
	recipe, _, valid := plannedHLSFile(strings.TrimPrefix(source, "/hls/"+data.ID+"/"))
	if !valid {
		return errPlaybackSeek
	}
	plan := data.Plan
	client, policy := browserPlaybackCapabilitiesForRequest(request, manager.settings, nil), viewerPlaybackPolicy(currentViewer(request))
	if plan.Mode == "direct" {
		intent := NetworkIntent{PreferCompatibility: true, AudioIndex: &plan.AudioIndex}
		if plan.SubtitleIndex >= 0 {
			intent.SubtitleIndex = &plan.SubtitleIndex
		}
		plan = playbackWithAutomaticSkip(facts, client, policy, intent, data.Markers, manager.settings.autoSkip())
	}
	selected, err := manager.exactSeekPlan(request.Context(), item, facts, client, policy, plan, recipe, position)
	if err != nil {
		return err
	}
	data.CompatibilityMode = selected.Mode
	data.CompatibilityLabel, data.CompatibilityDescription = playbackPresentation(selected)
	if data.Plan.Mode == "direct" {
		data.ModeLabel = data.CompatibilityLabel
		data.AdaptiveSource = ""
		if selected.Allowed {
			data.AdaptiveSource = hlsPlanURL(item.ID, selected)
		}
	} else {
		data.Plan = selected
		data.PlaybackLabel, data.PlaybackDescription = data.CompatibilityLabel, data.CompatibilityDescription
		data.Source = ""
		if selected.Allowed {
			data.Source = hlsPlanURL(item.ID, selected)
		}
	}
	return nil
}
