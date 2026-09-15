package mediaprobe

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func parseStreams(streams []probeStream) ([]string, VideoFacts, []AudioFacts, []SubtitleFacts, []AudioTrack) {
	parts, video, audioFacts, subtitleFacts, audio := make([]string, 0), VideoFacts{}, make([]AudioFacts, 0), make([]SubtitleFacts, 0), make([]AudioTrack, 0)
	for _, stream := range streams {
		if stream.CodecType != "video" {
			continue
		}
		if video.Codec == "" {
			video = videoFactsFor(stream)
		}
		parts = append(parts, fmt.Sprintf("%dx%d %s", stream.Width, stream.Height, strings.ToUpper(stream.CodecName)))
	}
	for _, stream := range streams {
		if stream.CodecType != "audio" {
			if stream.CodecType == "subtitle" {
				subtitleFacts = append(subtitleFacts, subtitleFactsFor(stream, len(subtitleFacts)))
			}
			continue
		}
		index := len(audio)
		audio = append(audio, AudioTrack{index, probeAudioLabel(stream, index) + " · " + strings.ToUpper(stream.CodecName)})
		audioFacts = append(audioFacts, audioFactsFor(stream, index))
	}
	return parts, video, audioFacts, subtitleFacts, audio
}

func videoFactsFor(stream probeStream) VideoFacts {
	depth, _ := strconv.Atoi(stream.BitsPerRawSample)
	if depth == 0 && strings.Contains(stream.PixelFormat, "10") {
		depth = 10
	}
	hdr, rotation, dvProfile, dvCompatibility := videoSideData(stream)
	return VideoFacts{Codec: lower(stream.CodecName), Profile: stream.Profile, Level: strconv.Itoa(stream.Level), PixelFormat: stream.PixelFormat, HDR: hdr, SampleAspectRatio: stream.SampleAspect, ColorRange: stream.ColorRange, ColorSpace: stream.ColorSpace, Transfer: stream.ColorTransfer, Primaries: stream.Primaries, Width: stream.Width, Height: stream.Height, BitDepth: depth, FrameRate: rational(stream.FrameRate), FieldOrder: lower(stream.FieldOrder), Rotation: rotation, DolbyVisionProfile: dvProfile, DolbyVisionCompatibility: dvCompatibility}
}

func audioFactsFor(stream probeStream, index int) AudioFacts {
	sampleRate, _ := strconv.Atoi(stream.SampleRate)
	role := "main"
	if stream.Disposition.VisualImpaired != 0 {
		role = "description"
	} else if stream.Disposition.Commentary != 0 || strings.Contains(lower(tagValue(stream.Tags, "title")), "commentary") {
		role = "commentary"
	}
	return AudioFacts{Index: index, SourceIndex: stream.Index, Codec: lower(stream.CodecName), Profile: stream.Profile, Language: lower(tagValue(stream.Tags, "language")), Role: role, ChannelLayout: stream.ChannelLayout, Channels: stream.Channels, SampleRate: sampleRate, Default: stream.Disposition.Default != 0, Forced: stream.Disposition.Forced != 0}
}

func subtitleFactsFor(stream probeStream, index int) SubtitleFacts {
	role := "translation"
	if stream.Disposition.HearingImpaired != 0 {
		role = "captions"
	} else if stream.Disposition.Commentary != 0 {
		role = "commentary"
	}
	return SubtitleFacts{Index: index, SourceIndex: stream.Index, Codec: lower(stream.CodecName), Language: lower(tagValue(stream.Tags, "language")), Role: role, Default: stream.Disposition.Default != 0, Forced: stream.Disposition.Forced != 0, Text: oneOf(lower(stream.CodecName), "ass", "ssa", "subrip", "srt", "mov_text", "text", "webvtt")}
}

func rational(value string) float64 {
	numerator, denominator, divided := strings.Cut(value, "/")
	left, leftErr := strconv.ParseFloat(numerator, 64)
	right, rightErr := strconv.ParseFloat(denominator, 64)
	if !divided || leftErr != nil || rightErr != nil || right == 0 {
		return left
	}
	return left / right
}

func oneOf(value string, allowed ...string) bool { return slices.Contains(allowed, value) }

func videoSideData(stream probeStream) (string, int, int, int) {
	hdr := ""
	rotation, dvProfile, dvCompatibility := 0, 0, 0
	switch lower(stream.ColorTransfer) {
	case "smpte2084":
		hdr = "hdr10"
	case "arib-std-b67":
		hdr = "hlg"
	}
	for _, side := range stream.SideData {
		name := lower(side.Type)
		if name == "display matrix" {
			rotation = side.Rotation
		}
		if side.DVProfile > 0 {
			dvProfile, dvCompatibility = side.DVProfile, side.DVCompatibility
		}
		if strings.Contains(name, "dovi") || strings.Contains(name, "dolby vision") {
			hdr = "dolby-vision"
		} else if strings.Contains(name, "hdr10+") || strings.Contains(name, "dynamic hdr plus") {
			hdr = "hdr10+"
		}
	}
	return hdr, rotation, dvProfile, dvCompatibility
}
