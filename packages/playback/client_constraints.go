package playback

import (
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type VideoLimit struct {
	Codec                 string
	Profiles              []string
	MaxLevel, MaxBitDepth int
}

type JellyfinCodecProfile struct {
	Type, Codec string
	Conditions  []JellyfinProfileCondition
}

type JellyfinProfileCondition struct {
	Condition, Property, Value string
	IsRequired                 bool
}

func validCodecProfiles(profiles []JellyfinCodecProfile) bool {
	if len(profiles) > 128 {
		return false
	}
	for _, profile := range profiles {
		if !oneOf(profile.Type, "Video", "Audio", "VideoAudio", "Photo", "Subtitle") || len(profile.Codec) > 1024 || len(profile.Conditions) > 64 {
			return false
		}
		for _, condition := range profile.Conditions {
			if !validProfileCondition(condition) {
				return false
			}
		}
	}
	return true
}

func matchesMediaProfile(profile JellyfinMediaProfile, container, video, audio, kind string) bool {
	return (profile.Type == "" || strings.EqualFold(profile.Type, kind)) &&
		(profile.Container == "" || Includes(AppendCSV(nil, profile.Container, true), container)) &&
		(video == "" || profile.VideoCodec == "" || Includes(AppendCSV(nil, profile.VideoCodec, false), video)) &&
		(audio == "" || profile.AudioCodec == "" || Includes(AppendCSV(nil, profile.AudioCodec, false), audio))
}

func directCombination(client ClientCapabilities, facts MediaFacts, audio AudioFacts) bool {
	if len(client.DirectProfiles) == 0 {
		return true
	}
	for _, profile := range client.DirectProfiles {
		if matchesMediaProfile(profile, facts.Container, facts.Video.Codec, audio.Codec, facts.Kind) {
			return true
		}
	}
	return false
}

func videoConstraints(client ClientCapabilities, facts MediaFacts, audio AudioFacts) bool {
	for _, limit := range client.VideoLimits {
		if limit.Codec != Lower(facts.Video.Codec) {
			continue
		}
		level, _ := strconv.Atoi(facts.Video.Level)
		if limit.MaxBitDepth > 0 && facts.Video.BitDepth > limit.MaxBitDepth || limit.MaxLevel > 0 && level > limit.MaxLevel || len(limit.Profiles) > 0 && facts.Video.Profile != "" && !Includes(limit.Profiles, facts.Video.Profile) {
			return false
		}
	}
	return codecConditions(client.CodecProfiles, "Video", facts, audio)
}

func codecConditions(profiles []JellyfinCodecProfile, kind string, facts MediaFacts, audio AudioFacts) bool {
	codec := facts.Video.Codec
	if kind == "Audio" {
		codec = audio.Codec
	}
	for _, profile := range profiles {
		if (!strings.EqualFold(profile.Type, kind) && (kind != "Audio" || facts.Kind != "video" || profile.Type != "VideoAudio")) || profile.Codec != "" && !Includes(AppendCSV(nil, profile.Codec, false), codec) {
			continue
		}
		if !matchesProfileConditions(profile.Conditions, facts, audio) {
			return false
		}
	}
	return true
}

func conditionValue(property string, facts MediaFacts, audio AudioFacts) (string, bool) {
	video := facts.Video
	values := map[string]string{
		"VideoProfile": video.Profile, "VideoLevel": video.Level,
		"Width": strconv.Itoa(video.Width), "Height": strconv.Itoa(video.Height), "VideoRotation": strconv.Itoa(video.Rotation),
		"VideoBitDepth": strconv.Itoa(video.BitDepth), "VideoWidth": strconv.Itoa(video.Width), "VideoHeight": strconv.Itoa(video.Height),
		"VideoFramerate": strconv.FormatFloat(video.FrameRate, 'f', -1, 64), "VideoRangeType": jellyfinRange(video.HDR),
		"AudioChannels": strconv.Itoa(audio.Channels), "AudioSampleRate": strconv.Itoa(audio.SampleRate),
		"IsInterlaced": strconv.FormatBool(Interlaced(video)), "IsAnamorphic": strconv.FormatBool(video.SampleAspectRatio != "" && video.SampleAspectRatio != "1:1"),
	}
	value, known := values[property]
	return value, known && value != ""
}

func matchesCondition(condition JellyfinProfileCondition, actual string) bool {
	switch condition.Condition {
	case "Equals":
		return strings.EqualFold(actual, condition.Value)
	case "NotEquals":
		return !strings.EqualFold(actual, condition.Value)
	case "EqualsAny":
		return Includes(strings.Split(condition.Value, "|"), actual)
	case "LessThanEqual", "GreaterThanEqual":
		left, leftOK := profileNumber(actual)
		right, rightOK := profileNumber(condition.Value)
		if !leftOK || !rightOK {
			return false
		}
		if condition.Condition == "LessThanEqual" {
			return left <= right
		}
		return left >= right
	}
	return false
}

func jellyfinRange(hdr string) string {
	return map[string]string{"": "SDR", "sdr": "SDR", "hdr10": "HDR10", "hdr10+": "HDR10Plus", "hlg": "HLG", "dolby-vision": "DOVI"}[hdr]
}

func explicitHDRSupport(profiles []JellyfinCodecProfile, video VideoFacts) bool {
	for _, profile := range profiles {
		if !strings.EqualFold(profile.Type, "Video") || profile.Codec != "" && !Includes(AppendCSV(nil, profile.Codec, false), video.Codec) {
			continue
		}
		for _, condition := range profile.Conditions {
			if condition.Property == "VideoRangeType" && oneOf(condition.Condition, "Equals", "EqualsAny") && matchesCondition(condition, jellyfinRange(video.HDR)) {
				return true
			}
		}
	}
	return false
}

func compatibleDelivery(client ClientCapabilities, plan PlaybackPlan) bool {
	if len(client.DeliveryProfiles) == 0 {
		return true
	}
	kind := "Video"
	if plan.VideoCodec == "" {
		kind = "Audio"
	}
	for _, profile := range client.DeliveryProfiles {
		if (profile.Protocol == "" || strings.EqualFold(profile.Protocol, "hls")) && matchesMediaProfile(profile, "mp4", plan.VideoCodec, plan.AudioCodec, kind) {
			return true
		}
	}
	return false
}

// Converted output is checked too: a decoder limited to a lower profile/level
// must not receive a conversion whose fixed encoder contract exceeds it.
func outputConstraints(client ClientCapabilities, plan PlaybackPlan, source MediaFacts) bool {
	output := source
	audio := selectedAudioFacts(source.Audio, plan.AudioIndex)
	if plan.Mode == "transcode" {
		if !validOutputCodec(client, plan, source.Video) {
			return false
		}
		output.Video = convertedVideoFacts(plan, source.Video)
	}
	if (plan.Mode == "transcode" || plan.Mode == "audio-transcode") && len(source.Audio) > 0 {
		audio.Codec, audio.Channels = "aac", 2
	}
	return (oneOf(source.Kind, "audio", "audiobook") || videoConstraints(client, output, audio)) && codecConditions(client.CodecProfiles, "Audio", output, audio) && (client.MaxAudioChannels == 0 || audio.Channels <= client.MaxAudioChannels)
}

func validProfileCondition(condition JellyfinProfileCondition) bool {
	if !oneOf(condition.Condition, "Equals", "NotEquals", "LessThanEqual", "GreaterThanEqual", "EqualsAny") || condition.Property == "" || len(condition.Property) > 64 || condition.Value == "" || len(condition.Value) > 1024 {
		return false
	}
	return validProfileValue(condition)
}

func validProfileValue(condition JellyfinProfileCondition) bool {
	if condition.Property == "VideoRotation" {
		for _, raw := range strings.Split(condition.Value, "|") {
			value, err := strconv.Atoi(raw)
			if err != nil || strconv.Itoa(value) != raw || value < -360 || value > 360 || value%90 != 0 {
				return false
			}
		}
		return condition.Condition == "EqualsAny" || !strings.Contains(condition.Value, "|")
	}
	if condition.Condition == "LessThanEqual" || condition.Condition == "GreaterThanEqual" {
		value, err := strconv.ParseFloat(condition.Value, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1e12 {
			return false
		}
	}
	return true
}

func matchesProfileConditions(conditions []JellyfinProfileCondition, facts MediaFacts, audio AudioFacts) bool {
	for _, condition := range conditions {
		value, known := conditionValue(condition.Property, facts, audio)
		if !known {
			if condition.IsRequired {
				return false
			}
			continue
		}
		if !matchesCondition(condition, value) {
			return false
		}
	}
	return true
}

func profileNumber(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(raw, 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func convertedVideoFacts(plan PlaybackPlan, source VideoFacts) VideoFacts {
	video := VideoFacts{Codec: plan.VideoCodec, Width: plan.Width, Height: plan.Height, BitDepth: 8, FrameRate: min(60, source.FrameRate), SampleAspectRatio: "1:1", FieldOrder: "progressive"}
	if plan.VideoCodec == "h264" {
		video.Profile, video.Level = "High", "42"
	}
	if plan.VideoCodec == "hevc" {
		video.Profile = "Main"
		if plan.ColorMode == "preserve" && source.HDR != "" && source.HDR != "sdr" {
			video.Profile, video.HDR, video.BitDepth = "Main 10", source.HDR, 10
		}
	}
	return video
}

func validOutputCodec(client ClientCapabilities, plan PlaybackPlan, source VideoFacts) bool {
	if !transcodepolicy.ValidCodec(plan.VideoCodec) || plan.VideoCodec == "auto" || source.HDR == "dolby-vision" && !oneOfInt(source.DolbyVisionCompatibility, 1, 4) {
		return false
	}
	if len(client.DeliveryProfiles) == 0 && len(client.VideoCodecs) > 0 && !Includes(client.VideoCodecs, plan.VideoCodec) {
		return false
	}
	return true
}

func selectedAudioFacts(tracks []AudioFacts, index int) AudioFacts {
	audio := AudioFacts{}
	for _, track := range tracks {
		if track.Index == index {
			audio = track
		}
	}
	return audio
}
