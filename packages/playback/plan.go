package playback

import (
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// DecisionPolicy contains the narrow product differences around track choice.
type DecisionPolicy struct {
	PreferCompatibleAudio bool
}

// Decide chooses the least-transforming representation allowed by every input.
func Decide(facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, intent NetworkIntent, product DecisionPolicy) PlaybackPlan {
	audioOnly := facts.Kind == "audio" || facts.Kind == "audiobook"
	if audioOnly {
		facts.Video = VideoFacts{}
	}
	plan := PlaybackPlan{Mode: "direct", Reason: "compatible", Container: Lower(facts.Container), VideoCodec: Lower(facts.Video.Codec), SubtitleMode: "none", ColorMode: "preserve", SubtitleIndex: -1, Width: facts.Video.Width, Height: facts.Video.Height}
	if !policy.AllowPlayback {
		return denied(plan, "playback-not-allowed")
	}
	audio := selectAudio(facts.Audio, client.AudioCodecs, intent, product)
	plan.AudioIndex, plan.AudioCodec = audio.Index, Lower(audio.Codec)
	if tracksUnavailable(facts, intent) {
		return denied(plan, "track-unavailable")
	}
	if !audioOnly {
		applySubtitle(&plan, facts.Subtitles, intent.SubtitleIndex, client)
	}
	limit := MinimumPositive(client.MaxBitrate, policy.MaxBitrate, intent.MaxBitrate)
	plan.MaxBitrate = limit
	compatible := playbackCompatibility(facts, client, audio, limit)
	if !compatible.hdr {
		plan.ColorMode = "tone-map-sdr"
	}
	choosePlaybackMode(&plan, intent, client, compatible)
	if audioOnly && plan.Mode != "direct" {
		plan.Mode, plan.Reason = "audio-transcode", "audio-compatibility"
	}
	return finalize(&plan, facts, client, policy, audioOnly, limit)
}

func denied(plan PlaybackPlan, reason string) PlaybackPlan {
	plan.Mode, plan.Reason = "denied", reason
	return plan
}

func tracksUnavailable(facts MediaFacts, intent NetworkIntent) bool {
	return intent.AudioIndex != nil && !hasAudio(facts.Audio, *intent.AudioIndex) || intent.SubtitleIndex != nil && !hasSubtitle(facts.Subtitles, *intent.SubtitleIndex)
}

func finalize(plan *PlaybackPlan, facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, audioOnly bool, limit int64) PlaybackPlan {
	if plan.Mode == "direct" {
		plan.Allowed = true
		return *plan
	}
	if !policy.AllowTranscode {
		return denied(*plan, "transcoding-not-allowed")
	}
	plan.Container = "mp4"
	applyPlaybackConversion(plan, facts, client, limit)
	if len(facts.Audio) == 0 {
		plan.AudioCodec = ""
	}
	if reason := conversionFailure(*plan, facts, audioOnly); reason != "" {
		return denied(*plan, reason)
	}
	if !compatibleDelivery(client, *plan) && !audioOnly && plan.Mode != "transcode" {
		plan.Mode, plan.Reason = "transcode", "delivery-format-unsupported"
		applyPlaybackConversion(plan, facts, client, limit)
	}
	if !compatibleDelivery(client, *plan) || !outputConstraints(client, *plan, facts) {
		return denied(*plan, "delivery-format-unsupported")
	}
	plan.Allowed = true
	return *plan
}

func conversionFailure(plan PlaybackPlan, facts MediaFacts, audioOnly bool) string {
	if audioOnly && plan.Mode != "direct" && len(facts.Audio) == 0 {
		return "conversion-unsupported"
	}
	if plan.Mode == "transcode" && (plan.VideoCodec == "" || dolbyVisionNeedsConversion(facts.Video)) {
		return "conversion-unsupported"
	}
	return ""
}

func dolbyVisionNeedsConversion(video VideoFacts) bool {
	return video.HDR == "dolby-vision" && video.DolbyVisionCompatibility != 1 && video.DolbyVisionCompatibility != 4
}

type compatibility struct {
	container, video, audio, size, bitrate, hdr bool
}

func applySubtitle(plan *PlaybackPlan, tracks []SubtitleFacts, requested *int, client ClientCapabilities) {
	subtitle, selected := selectSubtitle(tracks, requested)
	if !selected {
		return
	}
	plan.SubtitleIndex, plan.SubtitleSourceIndex, plan.SubtitleText = subtitle.Index, subtitle.SourceIndex, subtitle.Text
	plan.SubtitleExternal, plan.SubtitleExternalIndex = subtitle.External, subtitle.ExternalIndex
	switch {
	case !subtitle.Text || subtitle.External && !client.SupportsExternalSubtitles || !Includes(client.TextSubtitleCodecs, subtitle.Codec):
		plan.SubtitleMode = "burn-in"
	case subtitle.External:
		plan.SubtitleMode = "external"
	default:
		plan.SubtitleMode = "embedded"
	}
}

func playbackCompatibility(facts MediaFacts, client ClientCapabilities, audio AudioFacts, limit int64) compatibility { //nolint:cyclop // The compatibility vector keeps each independent media capability explicit.
	width, height := DisplayDimensions(facts.Video)
	hdr := Lower(facts.Video.HDR)
	return compatibility{
		container: Includes(client.Containers, facts.Container) && directCombination(client, facts, audio),
		video:     facts.Kind != "video" || facts.Video.Codec != "" && Includes(client.VideoCodecs, facts.Video.Codec) && videoConstraints(client, facts, audio),
		audio:     audio.Codec == "" || Includes(client.AudioCodecs, audio.Codec) && (client.MaxAudioChannels == 0 || audio.Channels <= client.MaxAudioChannels) && codecConditions(client.CodecProfiles, "Audio", facts, audio),
		size:      (client.MaxWidth == 0 || width <= client.MaxWidth) && (client.MaxHeight == 0 || height <= client.MaxHeight),
		bitrate:   limit == 0 || facts.Bitrate == 0 || facts.Bitrate <= limit,
		hdr:       hdr == "" || hdr == "sdr" || Includes(client.HDRFormats, hdr),
	}
}

func choosePlaybackMode(plan *PlaybackPlan, intent NetworkIntent, client ClientCapabilities, compatible compatibility) { //nolint:cyclop // Ordered playback policy remains below the repository quality ceiling.
	forceDirect := intent.ForceDirect || intent.PreferDirect && !intent.ForceTranscode
	needsVideo := intent.ForceTranscode || !compatible.video || !compatible.size || !compatible.bitrate || !compatible.hdr || plan.SubtitleMode == "burn-in"
	switch {
	case forceDirect:
		plan.Reason = "direct-requested"
		if intent.PreferDirect {
			plan.Reason = "direct-preferred"
		}
	case needsVideo:
		plan.Mode, plan.Reason = "transcode", playbackReason(compatible.video, compatible.size, compatible.bitrate, compatible.hdr, plan.SubtitleMode)
	case !compatible.audio:
		plan.Mode, plan.Reason = "audio-transcode", "audio-codec-unsupported"
	case intent.PreferCompatibility && client.SupportsRemux:
		plan.Mode, plan.Reason = "remux", "compatibility-requested"
	case !compatible.container && client.SupportsRemux:
		plan.Mode, plan.Reason = "remux", "container-unsupported"
	case !compatible.container:
		plan.Mode, plan.Reason = "transcode", "container-unsupported"
	}
}

func applyPlaybackConversion(plan *PlaybackPlan, facts MediaFacts, client ClientCapabilities, limit int64) {
	switch plan.Mode {
	case "transcode":
		plan.VideoCodec, plan.AudioCodec, plan.Adaptive = transcodeVideoCodec(client.TranscodeVideoCodecs), "aac", true
		width, height := DisplayDimensions(facts.Video)
		plan.Width, plan.Height = FitDimensions(width, height, MinimumPositiveInt(1920, client.MaxWidth), MinimumPositiveInt(1080, client.MaxHeight))
		if facts.Video.HDR != "" && facts.Video.HDR != "sdr" {
			if oneOf(facts.Video.HDR, "hdr10", "hlg") && plan.VideoCodec == "hevc" && Includes(client.HDRFormats, facts.Video.HDR) {
				plan.ColorMode = "preserve"
			} else {
				plan.ColorMode = "tone-map-sdr"
			}
		}
		plan.Qualities = AdaptiveQualities(plan.Width, plan.Height, min(60, facts.Video.FrameRate), limit)
	case "audio-transcode":
		plan.AudioCodec = "aac"
	}
}

func selectAudio(tracks []AudioFacts, codecs []string, intent NetworkIntent, product DecisionPolicy) AudioFacts { //nolint:cyclop // Ordered audio policy is intentionally evaluated in one pass.
	selected := AudioFacts{}
	for _, track := range tracks {
		if intent.AudioIndex != nil && track.Index == *intent.AudioIndex || intent.AudioIndex == nil && track.Default {
			selected = track
			break
		}
	}
	if selected.Codec == "" && len(tracks) > 0 {
		selected = tracks[0]
	}
	if product.PreferCompatibleAudio && intent.AudioIndex == nil && intent.PreferCompatibility && !Includes(codecs, selected.Codec) {
		for _, track := range tracks {
			if Includes(codecs, track.Codec) {
				return track
			}
		}
	}
	return selected
}

func selectSubtitle(tracks []SubtitleFacts, requested *int) (SubtitleFacts, bool) {
	if requested != nil {
		for _, track := range tracks {
			if track.Index == *requested {
				return track, true
			}
		}
	}
	return SubtitleFacts{}, false
}

func transcodeVideoCodec(codecs []string) string {
	for _, codec := range codecs {
		if codec != "auto" && transcodepolicy.ValidCodec(codec) {
			return codec
		}
	}
	if len(codecs) > 0 {
		return ""
	}
	return "h264"
}

func playbackReason(video, size, bitrate, hdr bool, subtitle string) string {
	switch {
	case !video:
		return "video-codec-unsupported"
	case !size:
		return "resolution-exceeds-client"
	case !bitrate:
		return "bitrate-exceeds-limit"
	case !hdr:
		return "hdr-unsupported"
	case subtitle == "burn-in":
		return "subtitle-burn-in-required"
	default:
		return "transcode-requested"
	}
}

// BrowserCapabilities returns Player's baseline browser contract.
func BrowserCapabilities() ClientCapabilities {
	return ClientCapabilities{Containers: []string{"mp4", "mov", "webm"}, VideoCodecs: []string{"h264", "vp8", "vp9", "av1"}, AudioCodecs: []string{"aac", "mp3", "opus", "vorbis"}, TextSubtitleCodecs: []string{"vtt", "webvtt", "srt", "subrip", "ass", "ssa", "mov_text", "text"}, HDRFormats: []string{"sdr"}, MaxAudioChannels: 2, VideoLimits: []VideoLimit{{Codec: "h264", Profiles: []string{"baseline", "constrained baseline", "main", "high"}, MaxBitDepth: 8, MaxLevel: 42}}, SupportsExternalSubtitles: true, SupportsRemux: true, MaxWidth: 3840, MaxHeight: 2160}
}

func NormalizeContainer(value string) string {
	value = Lower(value)
	switch value {
	case "matroska", "matroska,webm":
		return "mkv"
	case "quicktime", "mov,mp4,m4a,3gp,3g2,mj2":
		return "mp4"
	}
	return value
}

func Lower(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func Includes(values []string, value string) bool {
	value = Lower(value)
	for _, candidate := range values {
		if Lower(candidate) == value || candidate == "*" {
			return true
		}
	}
	return false
}

func MinimumPositive(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value > 0 && (result == 0 || value < result) {
			result = value
		}
	}
	return result
}

func MinimumPositiveInt(values ...int) int {
	result := 0
	for _, value := range values {
		if value > 0 && (result == 0 || value < result) {
			result = value
		}
	}
	return result
}
