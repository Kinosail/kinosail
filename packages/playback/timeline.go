package playback

import (
	"math"
	"slices"
	"sort"

	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/metadata"
)

// TimelineForAutomaticSkip returns one normalized presentation timeline.
func TimelineForAutomaticSkip(duration float64, values []markers.Marker, enabled []string) Timeline { //nolint:cyclop // Normalization must reject and merge every marker boundary in one pass.
	ranges := make([]Range, 0, len(values))
	for _, marker := range values {
		if !slices.Contains(enabled, marker.Type) || !automaticallySkippable(marker) || marker.Start < 0 || marker.End <= marker.Start || marker.Start >= duration {
			continue
		}
		ranges = append(ranges, Range{marker.Start, min(marker.End, duration)})
	}
	sort.Slice(ranges, func(left, right int) bool { return ranges[left].Start < ranges[right].Start })
	omitted := ranges[:0]
	for _, current := range ranges {
		if len(omitted) > 0 && current.Start <= omitted[len(omitted)-1].End {
			omitted[len(omitted)-1].End = max(omitted[len(omitted)-1].End, current.End)
			continue
		}
		omitted = append(omitted, current)
	}
	presented := duration
	for _, value := range omitted {
		presented -= value.End - value.Start
	}
	if presented <= 0 {
		return Timeline{SourceDuration: duration, Duration: duration}
	}
	return Timeline{SourceDuration: duration, Duration: max(0, presented), Omitted: omitted}
}

// AutomaticSkipTypes returns marker types whose full set is trusted.
func AutomaticSkipTypes(values []markers.Marker, enabled []string) []string {
	result := make([]string, 0, len(enabled))
	for _, markerType := range enabled {
		found, safe := false, true
		for _, marker := range values {
			if marker.Type == markerType {
				found, safe = true, safe && automaticallySkippable(marker)
			}
		}
		if found && safe {
			result = append(result, markerType)
		}
	}
	return result
}

// AutomaticStart returns a trusted opening marker offset for new playback.
func AutomaticStart(current, duration float64, values []markers.Marker, enabled []string) float64 { //nolint:cyclop,gocognit // Intro trust decisions remain one auditable sequence.
	if !finiteTimelineValue(current) || current < 0 {
		return 0
	}
	if current > 0 || !finiteTimelineValue(duration) || duration <= 10 || !slices.Contains(AutomaticSkipTypes(values, enabled), "intro") {
		return current
	}
	offset := 0.0
	for _, marker := range values {
		if marker.Type != "intro" {
			continue
		}
		if !finiteTimelineValue(marker.Start) || !finiteTimelineValue(marker.End) || marker.Start < 0 || marker.End <= marker.Start {
			return current
		}
		if marker.Start == 0 {
			if marker.End >= duration-10 || offset != 0 && marker.End != offset {
				return current
			}
			offset = marker.End
		}
	}
	return offset
}

// RemainingMarkers maps non-omitted markers onto one presentation timeline.
func RemainingMarkers(values []markers.Marker, enabled []string, timeline Timeline) []markers.Marker {
	result := make([]markers.Marker, 0, len(values))
	for _, marker := range values {
		if slices.Contains(enabled, marker.Type) && automaticallySkippable(marker) {
			continue
		}
		marker.Start, marker.End = timeline.PresentationTime(marker.Start), timeline.PresentationTime(marker.End)
		if marker.End > marker.Start {
			result = append(result, marker)
		}
	}
	return result
}

// TimelineChapters maps chapters onto one presentation timeline.
func TimelineChapters(timeline Timeline, values []metadata.Chapter) []metadata.Chapter {
	result := make([]metadata.Chapter, 0, len(values))
	for _, chapter := range values {
		chapter.Start, chapter.End = timeline.PresentationTime(chapter.Start), timeline.PresentationTime(chapter.End)
		if chapter.End > chapter.Start {
			chapter.Index = len(result)
			result = append(result, chapter)
		}
	}
	return result
}

// DecideWithAutomaticSkip applies Player's server-side marker policy.
func DecideWithAutomaticSkip(facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, intent NetworkIntent, values []markers.Marker, enabled []string, product DecisionPolicy) PlaybackPlan { //nolint:cyclop // The delivery fallback sequence stays explicit and below the required project limit.
	if intent.PreferDirect {
		return Decide(facts, client, policy, intent, product)
	}
	timeline := TimelineForAutomaticSkip(facts.Duration, values, enabled)
	if len(timeline.Omitted) == 0 {
		return Decide(facts, client, policy, intent, product)
	}
	if !policy.AllowTranscode {
		plan := Decide(facts, client, policy, intent, product)
		plan.MarkerMode = "unavailable"
		return plan
	}
	intent.ForceDirect = false
	if aligned, ok := randomAccessTimeline(timeline, facts.RandomAccess, facts.Video.FrameRate); ok && !intent.ForceTranscode && slices.Contains([]string{"h264", "hevc"}, Lower(facts.Video.Codec)) {
		plan := Decide(facts, client, policy, intent, product)
		if plan.Mode == "direct" || plan.Mode == "remux" || plan.Mode == "audio-transcode" {
			if len(facts.Audio) > 0 {
				plan.Mode, plan.AudioCodec = "audio-transcode", "aac"
			} else {
				plan.Mode = "remux"
			}
			plan.Container, plan.Reason, plan.MarkerMode, plan.Timeline = "mp4", "automatic-marker-skip-remux", "server", aligned
			return plan
		}
	}
	intent.ForceDirect, intent.ForceTranscode = false, true
	plan := Decide(facts, client, policy, intent, product)
	plan.Reason, plan.MarkerMode, plan.Timeline = "automatic-marker-skip", "server", timeline
	return plan
}

// JellyfinMarkers returns markers on the timeline visible to one Jellyfin viewer.
func JellyfinMarkers(duration float64, values []markers.Marker, enabled []string, serverTimeline bool) []markers.Marker {
	timeline := TimelineForAutomaticSkip(duration, values, enabled)
	if serverTimeline && len(timeline.Omitted) > 0 {
		return RemainingMarkers(values, enabled, timeline)
	}
	return values
}

type TimelineResponse interface {
	SetPlaybackTimeline(float64, float64, []metadata.Chapter, []markers.Marker, []string, string)
}

// ApplyAPIPlaybackTimeline projects source media onto the timeline returned to API clients.
func ApplyAPIPlaybackTimeline(response TimelineResponse, plan PlaybackPlan, start, duration float64, chapters []metadata.Chapter, values []markers.Marker, enabled, autoSkip []string, progressToken func() string) {
	if plan.MarkerMode == "server" {
		response.SetPlaybackTimeline(plan.Timeline.Duration, plan.Timeline.PresentationTime(start), TimelineChapters(plan.Timeline, chapters), RemainingMarkers(values, enabled, plan.Timeline), []string{}, progressToken())
		return
	}
	response.SetPlaybackTimeline(duration, AutomaticStart(start, duration, values, enabled), chapters, values, autoSkip, "")
}

func automaticallySkippable(marker markers.Marker) bool {
	return marker.Source == "manual" || marker.Source == "chapter" || marker.Source == "fingerprint"
}

func randomAccessTimeline(timeline Timeline, points []float64, frameRate float64) (Timeline, bool) {
	if len(points) == 0 {
		return Timeline{}, false
	}
	tolerance := 0.002
	if frameRate > 0 {
		tolerance = max(tolerance, 0.5/frameRate)
	}
	aligned := Timeline{SourceDuration: timeline.SourceDuration, Duration: timeline.SourceDuration, Omitted: make([]Range, 0, len(timeline.Omitted))}
	for _, value := range timeline.Omitted {
		start, startOK := randomAccessPoint(value.Start, timeline.SourceDuration, points, tolerance)
		end, endOK := randomAccessPoint(value.End, timeline.SourceDuration, points, tolerance)
		if !startOK || !endOK || end <= start {
			return Timeline{}, false
		}
		aligned.Omitted = append(aligned.Omitted, Range{start, end})
		aligned.Duration -= end - start
	}
	return aligned, aligned.Duration > 0
}

func randomAccessPoint(value, duration float64, points []float64, tolerance float64) (float64, bool) {
	if math.Abs(value) <= tolerance {
		return 0, true
	}
	if math.Abs(value-duration) <= tolerance {
		return duration, true
	}
	nearest, distance := 0.0, math.MaxFloat64
	for _, point := range points {
		if current := math.Abs(point - value); current < distance {
			nearest, distance = point, current
		}
	}
	return nearest, distance <= tolerance
}

func finiteTimelineValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
