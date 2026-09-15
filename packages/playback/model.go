// Package playback defines Player's media-delivery policy for Kinosail apps.
package playback

// MediaFacts is the normalized source description consumed by playback adapters.
type MediaFacts struct {
	Kind         string          `json:"kind"`
	FileVersion  string          `json:"fileVersion"`
	Container    string          `json:"container"`
	Bitrate      int64           `json:"bitrate,omitempty"`
	Duration     float64         `json:"duration,omitempty"`
	Seekable     bool            `json:"seekable"`
	Video        VideoFacts      `json:"video"`
	Audio        []AudioFacts    `json:"audio"`
	Subtitles    []SubtitleFacts `json:"subtitles"`
	RandomAccess []float64       `json:"-"`
}

type VideoFacts struct {
	Codec, Profile, Level, PixelFormat, HDR                        string
	SampleAspectRatio, ColorRange, ColorSpace, Transfer, Primaries string
	Width, Height, BitDepth                                        int
	FrameRate                                                      float64
	FieldOrder                                                     string
	Rotation                                                       int
	DolbyVisionProfile, DolbyVisionCompatibility                   int
}

type AudioFacts struct {
	Index, SourceIndex                            int
	Codec, Profile, Language, Role, ChannelLayout string
	Channels, SampleRate                          int
	Default, Forced                               bool
}

type SubtitleFacts struct {
	Index, SourceIndex              int
	Codec, Language, Role           string
	Default, Forced, Text, External bool
	ExternalIndex                   int
}

type ClientCapabilities struct {
	Containers, VideoCodecs, AudioCodecs, TextSubtitleCodecs, HDRFormats []string
	TranscodeVideoCodecs                                                 []string
	DirectProfiles                                                       []JellyfinMediaProfile
	DeliveryProfiles                                                     []JellyfinMediaProfile
	CodecProfiles                                                        []JellyfinCodecProfile
	VideoLimits                                                          []VideoLimit
	MaxAudioChannels                                                     int
	MaxWidth, MaxHeight                                                  int
	MaxBitrate                                                           int64
	SupportsExternalSubtitles, SupportsRemux                             bool
}

type ViewerPolicy struct {
	AllowPlayback, AllowTranscode bool
	MaxBitrate                    int64
}

type NetworkIntent struct {
	MaxBitrate                        int64
	ForceDirect, ForceTranscode       bool
	PreferDirect, PreferCompatibility bool
	AudioIndex, SubtitleIndex         *int
}

type PlaybackPlan struct {
	Allowed               bool              `json:"allowed"`
	Mode                  string            `json:"mode"`
	Reason                string            `json:"reason"`
	Container             string            `json:"container,omitempty"`
	VideoCodec            string            `json:"videoCodec,omitempty"`
	AudioCodec            string            `json:"audioCodec,omitempty"`
	SubtitleMode          string            `json:"subtitleMode"`
	ColorMode             string            `json:"colorMode"`
	AudioIndex            int               `json:"audioIndex"`
	SubtitleIndex         int               `json:"subtitleIndex"`
	SubtitleSourceIndex   int               `json:"subtitleSourceIndex,omitempty"`
	SubtitleText          bool              `json:"subtitleText,omitempty"`
	SubtitleExternal      bool              `json:"subtitleExternal,omitempty"`
	SubtitleExternalIndex int               `json:"subtitleExternalIndex,omitempty"`
	MaxBitrate            int64             `json:"maxBitrate,omitempty"`
	Width                 int               `json:"width,omitempty"`
	Height                int               `json:"height,omitempty"`
	Adaptive              bool              `json:"adaptive"`
	Qualities             []PlaybackQuality `json:"qualities,omitempty"`
	MarkerMode            string            `json:"markerMode,omitempty"`
	Timeline              Timeline          `json:"timeline,omitempty"`
}

type PlaybackQuality struct {
	Label     string  `json:"label"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Bitrate   int64   `json:"bitrate"`
	FrameRate float64 `json:"frameRate,omitempty"`
}

type Range struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Timeline struct {
	SourceDuration float64 `json:"sourceDuration"`
	Duration       float64 `json:"duration"`
	Omitted        []Range `json:"omitted"`
}

// SourceTime maps a presentation position to the source timeline.
func (timeline Timeline) SourceTime(presentation float64) float64 {
	presentation = max(0, min(presentation, timeline.Duration))
	removed := 0.0
	for _, value := range timeline.Omitted {
		if presentation < value.Start-removed {
			break
		}
		removed += value.End - value.Start
	}
	return min(timeline.SourceDuration, presentation+removed)
}

// PresentationTime maps a source position to the presented timeline.
func (timeline Timeline) PresentationTime(source float64) float64 {
	source = max(0, min(source, timeline.SourceDuration))
	removed := 0.0
	for _, value := range timeline.Omitted {
		if source < value.Start {
			break
		}
		if source < value.End {
			return value.Start - removed
		}
		removed += value.End - value.Start
	}
	return max(0, source-removed)
}

func hasSubtitle(tracks []SubtitleFacts, index int) bool {
	for _, track := range tracks {
		if track.Index == index {
			return true
		}
	}
	return false
}

func hasAudio(tracks []AudioFacts, index int) bool {
	for _, track := range tracks {
		if track.Index == index {
			return true
		}
	}
	return false
}
