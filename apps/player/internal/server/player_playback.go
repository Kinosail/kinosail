package server

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

func applyPlayback(request *http.Request, settings *settingsStore, facts MediaFacts, data *playerData) { //nolint:cyclop,gocognit // Playback selection and its fallback/shortened projection are one adapter operation.
	if data.Kind != "video" {
		data.ModeURL = ""
		return
	}
	data.AdminMarkers = append([]playbackMarker(nil), data.Markers...)
	viewer := currentViewer(request)
	data.AutoSkip = strings.Join(automaticSkipSelection(data.Markers, settings.autoSkip()), ",")
	intent := playbackModeIntent(settings)
	data.PlaybackPolicy = settings.playbackMode()
	if request.URL.Query().Has("direct") {
		intent.ForceDirect, intent.ForceTranscode, intent.PreferDirect, intent.PreferCompatibility = true, false, false, false
		data.PlaybackPolicy = "direct"
		data.PlaybackOverride = true
	}
	if request.URL.Query().Has("compatible") {
		intent.ForceDirect, intent.ForceTranscode, intent.PreferDirect, intent.PreferCompatibility = false, false, false, true
		data.PlaybackPolicy = "compatible"
		data.PlaybackOverride = true
	}
	if value := request.URL.Query().Get("audio"); value != "" {
		if track, err := audioTrackIndex(value); err == nil {
			intent.AudioIndex = &track
		}
	}
	if value := request.URL.Query().Get("subtitle"); value != "" {
		if track, err := audioTrackIndex(value); err == nil {
			intent.SubtitleIndex = &track
		}
	}
	policy := viewerPlaybackPolicy(viewer)
	policy.AllowTranscode = policy.AllowTranscode && !intent.ForceDirect
	client := browserPlaybackCapabilitiesForRequest(request, settings, nil)
	data.Plan = playbackWithAutomaticSkip(facts, client, policy, intent, data.Markers, settings.autoSkip())
	data.PlaybackLabel, data.PlaybackDescription = playbackPresentation(data.Plan)
	if data.Plan.Mode == "denied" {
		data.ModeURL = ""
		return
	}
	applyPlaybackSources(data, facts, client, viewer, intent, settings)
	applyPlaybackTimeline(data, facts, settings)
}

func applyPlaybackSources(data *playerData, facts MediaFacts, client ClientCapabilities, viewer viewerProfile, intent NetworkIntent, settings *settingsStore) {
	if data.Plan.Mode != "direct" {
		data.Source, data.ModeURL, data.ModeLabel, data.HLS = hlsPlanURL(data.ID, data.Plan), "?direct=1", "Direct Play", true
		data.DirectSource, data.DirectType = "/media/"+data.ID, directMediaType(data.Path, facts)
		data.CompatibilityMode, data.CompatibilityLabel, data.CompatibilityDescription = data.Plan.Mode, data.PlaybackLabel, data.PlaybackDescription
	} else if viewerPlaybackPolicy(viewer).AllowTranscode {
		fallback := intent
		fallback.ForceDirect, fallback.PreferDirect, fallback.PreferCompatibility = false, false, true
		compatible := playbackWithAutomaticSkip(facts, client, viewerPlaybackPolicy(viewer), fallback, data.Markers, settings.autoSkip())
		data.CompatibilityMode = compatible.Mode
		data.CompatibilityLabel, data.CompatibilityDescription = playbackPresentation(compatible)
		data.ModeLabel = data.CompatibilityLabel
		if data.PlaybackPolicy == "automatic" {
			data.AdaptiveSource, data.FallbackSource = hlsPlanURL(data.ID, compatible), "?compatible=1"
		}
		data.DirectSource, data.DirectType = "/media/"+data.ID, directMediaType(data.Path, facts)
	}
}

func applyPlaybackTimeline(data *playerData, facts MediaFacts, settings *settingsStore) {
	if data.Plan.MarkerMode == "server" {
		data.PlaybackToken, data.AutoSkip, data.ModeURL, data.DirectSource = recipeFor(data.Plan).token(), "", "", ""
		data.Start, data.Duration, data.Chapters = data.Plan.Timeline.PresentationTime(data.Start), data.Plan.Timeline.Duration, timelineChapters(data.Plan.Timeline, data.Chapters)
		data.Markers = remainingPlaybackMarkers(data.Markers, settings.autoSkip(), data.Plan.Timeline)
		for index := range data.Tracks {
			data.Tracks[index].Source += "?playbackToken=" + url.QueryEscape(data.PlaybackToken)
		}
	} else {
		data.Start = automaticStartOffset(data.Start, facts.Duration, data.Markers, settings.autoSkip())
	}
}

func playbackModeLabel(mode string) string {
	if label := map[string]string{"direct": "Direct Play", "remux": "Remux", "audio-transcode": "Transcoding audio", "transcode": "Transcoding video"}[mode]; label != "" {
		return label
	}
	return "Playback unavailable"
}

func playbackPresentation(plan PlaybackPlan) (string, string) {
	label := playbackModeLabel(plan.Mode)
	description := map[string]string{
		"direct":          "Original video and audio. No conversion.",
		"remux":           "Repackages the original video and audio without conversion.",
		"audio-transcode": "Keeps the original video. Converts only the selected audio.",
	}[plan.Mode]
	if plan.VideoCodec == "" && plan.Mode == "audio-transcode" {
		description = "Converts audio for this device."
	}
	if plan.Mode == "transcode" {
		description = map[string]string{
			"video-codec-unsupported":   "This device cannot decode the original video.",
			"resolution-exceeds-client": "The original resolution exceeds this device limit.",
			"bitrate-exceeds-limit":     "The original exceeds an explicit streaming limit.",
			"hdr-unsupported":           "This device cannot display the original HDR format.",
			"subtitle-burn-in-required": "The selected subtitles must be added to the video.",
			"automatic-marker-skip":     "Exact automatic skipping requires video conversion.",
		}[plan.Reason]
		if description == "" {
			description = "Converts the original video for this device."
		}
	}
	if description == "" {
		description = "Playback is unavailable."
	}
	return label, description
}

func directMediaType(path string, facts MediaFacts) string { //nolint:cyclop // Container-specific codec declarations stay conservative and truthful.
	extension := lower(filepath.Ext(path))
	if extension == ".mkv" {
		return "video/x-matroska"
	}
	container := map[string]string{".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm"}[extension]
	if container == "" {
		return ""
	}
	video := directVideoCodec(facts.Video, extension == ".webm")
	audioFacts := AudioFacts{}
	if len(facts.Audio) > 0 {
		audioFacts = facts.Audio[0]
	}
	audio := map[string]string{"mp3": "mp3", "ac3": "ac-3", "eac3": "ec-3"}[audioFacts.Codec]
	if extension == ".webm" {
		audio = map[string]string{"opus": "opus", "vorbis": "vorbis"}[audioFacts.Codec]
	} else if audioFacts.Codec == "aac" && (audioFacts.Profile == "" || strings.EqualFold(audioFacts.Profile, "LC")) {
		audio = "mp4a.40.2"
	}
	if video == "" {
		return ""
	}
	if len(facts.Audio) > 0 && audio == "" {
		return ""
	}
	if audio != "" {
		video += ", " + audio
	}
	return container + `; codecs="` + video + `"`
}

func directVideoCodec(video VideoFacts, webm bool) string {
	if webm {
		return map[string]string{"vp8": "vp8", "vp9": "vp9", "av1": "av01"}[video.Codec]
	}
	if video.Codec == "av1" {
		return "av01"
	}
	if video.Codec != "h264" {
		return ""
	}
	profile := map[string]string{"constrained baseline": "42e0", "baseline": "4200", "main": "4d00", "high": "6400"}[lower(video.Profile)]
	level, err := strconv.Atoi(video.Level)
	if profile == "" || err != nil || level < 1 || level > 255 {
		return ""
	}
	return "avc1." + profile + fmt.Sprintf("%02x", level)
}
