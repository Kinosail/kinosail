package playback

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	maximumJellyfinRequestBytes     = 1 << 20
	maximumJellyfinStreamingBitrate = 1_000_000_000_000
)

type JellyfinPlaybackRequest struct {
	DeviceProfile       JellyfinDeviceProfile `json:"DeviceProfile"`
	MaxStreamingBitrate int64                 `json:"MaxStreamingBitrate"`
	AudioStreamIndex    *int                  `json:"AudioStreamIndex"`
	SubtitleStreamIndex *int                  `json:"SubtitleStreamIndex"`
}

type JellyfinDeviceProfile struct {
	DirectPlayProfiles   []JellyfinMediaProfile    `json:"DirectPlayProfiles"`
	DirectStreamProfiles []JellyfinMediaProfile    `json:"DirectStreamProfiles"`
	TranscodingProfiles  []JellyfinMediaProfile    `json:"TranscodingProfiles"`
	SubtitleProfiles     []JellyfinSubtitleProfile `json:"SubtitleProfiles"`
	CodecProfiles        []JellyfinCodecProfile    `json:"CodecProfiles"`
}

type JellyfinMediaProfile struct {
	Container, Type, VideoCodec, AudioCodec, Protocol string
}

type JellyfinSubtitleProfile struct{ Format, Method string }

func ReadJellyfinPlaybackRequest(request *http.Request) (JellyfinPlaybackRequest, bool, bool) { //nolint:cyclop // Strict decoding and semantic validation share one request boundary.
	if request == nil || request.Method != http.MethodPost || request.Body == nil {
		return JellyfinPlaybackRequest{}, false, true
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, maximumJellyfinRequestBytes+1))
	if err != nil || len(data) > maximumJellyfinRequestBytes {
		return JellyfinPlaybackRequest{}, false, false
	}
	var input JellyfinPlaybackRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || !ValidJellyfinPlaybackRequest(input) {
		return JellyfinPlaybackRequest{}, false, false
	}
	provided := len(input.DeviceProfile.DirectPlayProfiles) > 0 || len(input.DeviceProfile.TranscodingProfiles) > 0 || len(input.DeviceProfile.DirectStreamProfiles) > 0
	return input, provided, true
}

func ValidJellyfinPlaybackRequest(input JellyfinPlaybackRequest) bool {
	return input.MaxStreamingBitrate >= 0 && input.MaxStreamingBitrate <= maximumJellyfinStreamingBitrate && validJellyfinTrack(input.AudioStreamIndex) &&
		validJellyfinTrack(input.SubtitleStreamIndex) && validJellyfinDeviceProfile(input.DeviceProfile)
}

func validJellyfinTrack(index *int) bool { return index == nil || *index >= 0 && *index <= 1024 }

func validJellyfinDeviceProfile(profile JellyfinDeviceProfile) bool {
	return validJellyfinMediaProfiles(profile.DirectPlayProfiles) && validJellyfinMediaProfiles(profile.DirectStreamProfiles) &&
		validJellyfinMediaProfiles(profile.TranscodingProfiles) && validJellyfinSubtitleProfiles(profile.SubtitleProfiles) && validCodecProfiles(profile.CodecProfiles)
}

func validJellyfinMediaProfiles(profiles []JellyfinMediaProfile) bool {
	if len(profiles) > 128 {
		return false
	}
	for _, profile := range profiles {
		if len(profile.Container) > 1024 || len(profile.Type) > 64 || len(profile.VideoCodec) > 1024 || len(profile.AudioCodec) > 1024 || len(profile.Protocol) > 64 {
			return false
		}
	}
	return true
}

func validJellyfinSubtitleProfiles(profiles []JellyfinSubtitleProfile) bool {
	if len(profiles) > 128 {
		return false
	}
	for _, profile := range profiles {
		if len(profile.Format) > 1024 || len(profile.Method) > 64 {
			return false
		}
	}
	return true
}

func JellyfinCapabilities(profile JellyfinDeviceProfile, facts MediaFacts) ClientCapabilities { //nolint:cyclop // Profile fields map explicitly onto the canonical planner.
	capabilities := ClientCapabilities{HDRFormats: []string{"sdr"}, DirectProfiles: profile.DirectPlayProfiles, CodecProfiles: profile.CodecProfiles, DeliveryProfiles: append(append([]JellyfinMediaProfile(nil), profile.DirectStreamProfiles...), profile.TranscodingProfiles...)}
	for _, value := range profile.DirectPlayProfiles {
		if !strings.EqualFold(value.Type, facts.Kind) && (facts.Kind != "video" || !strings.EqualFold(value.Type, "Video")) {
			continue
		}
		capabilities.Containers = AppendCSV(capabilities.Containers, value.Container, true)
		capabilities.VideoCodecs = AppendCSV(capabilities.VideoCodecs, value.VideoCodec, false)
		capabilities.AudioCodecs = AppendCSV(capabilities.AudioCodecs, value.AudioCodec, false)
	}
	if facts.Video.HDR != "" && explicitHDRSupport(profile.CodecProfiles, facts.Video) {
		capabilities.HDRFormats = append(capabilities.HDRFormats, facts.Video.HDR)
	}
	capabilities.SupportsRemux = len(profile.DirectStreamProfiles) > 0
	for _, value := range profile.TranscodingProfiles {
		capabilities.TranscodeVideoCodecs = AppendCSV(capabilities.TranscodeVideoCodecs, value.VideoCodec, false)
	}
	for _, value := range profile.SubtitleProfiles {
		if strings.EqualFold(value.Method, "External") || strings.EqualFold(value.Method, "Embed") {
			capabilities.TextSubtitleCodecs = AppendCSV(capabilities.TextSubtitleCodecs, value.Format, false)
			capabilities.SupportsExternalSubtitles = capabilities.SupportsExternalSubtitles || strings.EqualFold(value.Method, "External")
		}
	}
	return capabilities
}

func AppendCSV(target []string, value string, containers bool) []string {
	for part := range strings.SplitSeq(value, ",") {
		part = Lower(part)
		if containers {
			part = NormalizeContainer(part)
		}
		if part != "" {
			target = append(target, part)
		}
	}
	return target
}

type JellyfinMediaSourceOptions struct {
	PlaySessionID, Token     string
	QueryOnDirectPath        bool
	TranscodingProtocolField string
}

func JellyfinMediaSource(item library.Item, facts MediaFacts, plan PlaybackPlan, options JellyfinMediaSourceOptions) (map[string]any, error) { //nolint:cyclop // The Jellyfin wire object is built from one validated playback plan.
	if len(options.PlaySessionID) > 256 || len(options.Token) > 256 || options.TranscodingProtocolField != "TranscodingProtocol" && options.TranscodingProtocolField != "TranscodingSubProtocol" {
		return nil, errors.New("jellyfin media source options are invalid")
	}
	values := url.Values{}
	if options.PlaySessionID != "" {
		values.Set("playSessionId", options.PlaySessionID)
	}
	if options.Token != "" {
		values.Set("api_key", options.Token)
	}
	query := values.Encode()
	if query != "" {
		query = "?" + query
	}
	streams := JellyfinMediaStreams(item, facts, query)
	for index := range item.Subtitles {
		streams = append(streams, map[string]any{
			"Index": index + 1, "Type": "Subtitle", "Codec": "vtt", "DisplayTitle": "Subtitle " + strconv.Itoa(index+1),
			"IsExternal": true, "IsTextSubtitleStream": true, "SupportsExternalStream": true, "DeliveryMethod": "External",
			"DeliveryUrl": "/Videos/" + JellyfinID(item.ID) + "/" + JellyfinID(item.ID) + "/Subtitles/" + strconv.Itoa(index) + "/Stream.vtt" + query,
		})
	}
	direct, transcode := plan.Mode == "direct", plan.Allowed && plan.Mode != "direct"
	directStream := plan.Mode == "remux" || plan.Mode == "audio-transcode"
	defaultAudio := -1
	for _, track := range facts.Audio {
		if track.Index == plan.AudioIndex {
			defaultAudio = track.SourceIndex
			break
		}
	}
	pathQuery := ""
	if options.QueryOnDirectPath {
		pathQuery = query
	}
	result := map[string]any{
		"Id": JellyfinID(item.ID), "ETag": JellyfinID(item.ID), "Name": item.Title, "Protocol": "Http", "Type": "Default",
		"Path": "/Videos/" + JellyfinID(item.ID) + "/stream" + pathQuery, "Container": facts.Container, "Size": item.Size,
		"SupportsDirectPlay": direct, "SupportsDirectStream": directStream, "SupportsTranscoding": transcode, "MediaStreams": streams,
		"DefaultAudioStreamIndex": defaultAudio,
	}
	duration := facts.Duration
	if plan.MarkerMode == "server" {
		duration = plan.Timeline.Duration
	}
	result["RunTimeTicks"] = int64(duration * 1e7)
	if transcode {
		result["TranscodingContainer"], result[options.TranscodingProtocolField] = "mp4", "hls"
		result["TranscodingUrl"] = "/Videos/" + JellyfinID(item.ID) + "/p/" + RecipeFor(plan).Token() + "/index.m3u8" + query
	}
	return result, nil
}

func JellyfinMediaStreams(item library.Item, facts MediaFacts, query string) []any {
	streams := make([]any, 0, 1+len(facts.Audio)+len(facts.Subtitles))
	if facts.Video.Codec != "" {
		stream := map[string]any{"Index": 0, "Type": "Video", "Codec": facts.Video.Codec, "Profile": facts.Video.Profile, "Width": facts.Video.Width, "Height": facts.Video.Height, "BitDepth": facts.Video.BitDepth, "VideoRangeType": map[bool]string{true: "SDR", false: strings.ToUpper(facts.Video.HDR)}[facts.Video.HDR == ""], "IsDefault": true}
		if level, err := strconv.ParseFloat(facts.Video.Level, 64); err == nil {
			stream["Level"] = level
		}
		streams = append(streams, stream)
	}
	for _, track := range facts.Audio {
		streams = append(streams, map[string]any{"Index": track.SourceIndex, "Type": "Audio", "Codec": track.Codec, "Profile": track.Profile, "Language": track.Language, "DisplayTitle": track.Role, "Channels": track.Channels, "ChannelLayout": track.ChannelLayout, "SampleRate": track.SampleRate, "IsDefault": track.Default, "IsForced": track.Forced})
	}
	for _, track := range facts.Subtitles {
		if track.External {
			continue
		}
		stream := map[string]any{"Index": track.SourceIndex, "Type": "Subtitle", "Codec": track.Codec, "Language": track.Language, "DisplayTitle": track.Role, "IsDefault": track.Default, "IsForced": track.Forced, "IsExternal": false, "IsTextSubtitleStream": track.Text}
		if track.Text {
			stream["SupportsExternalStream"], stream["DeliveryMethod"] = true, "External"
			stream["DeliveryUrl"] = "/Videos/" + JellyfinID(item.ID) + "/" + JellyfinID(item.ID) + "/Subtitles/" + strconv.Itoa(track.SourceIndex) + "/Stream.vtt" + query
		}
		streams = append(streams, stream)
	}
	return streams
}

func JellyfinID(id string) string { return id + "0000000000000000" }

type JellyfinPlaybackHandlers struct {
	PlaybackInfo, Image, Stream, Subtitle, Progress, UserData, Played, Favorite http.HandlerFunc
}

func RegisterJellyfinPlayback(mux *http.ServeMux, handlers JellyfinPlaybackHandlers) error {
	if mux == nil || handlers.PlaybackInfo == nil || handlers.Image == nil || handlers.Stream == nil || handlers.Subtitle == nil || handlers.Progress == nil || handlers.UserData == nil || handlers.Played == nil || handlers.Favorite == nil {
		return errors.New("jellyfin playback handlers are invalid")
	}
	mux.HandleFunc("GET /Items/{id}/PlaybackInfo", handlers.PlaybackInfo)
	mux.HandleFunc("POST /Items/{id}/PlaybackInfo", handlers.PlaybackInfo)
	mux.HandleFunc("GET /Items/{id}/Images/{type}", handlers.Image)
	mux.HandleFunc("GET /Items/{id}/Images/{type}/{index}", handlers.Image)
	mux.HandleFunc("GET /Items/{id}/File", handlers.Stream)
	mux.HandleFunc("GET /Items/{id}/Download", handlers.Stream)
	mux.HandleFunc("GET /Videos/{id}/{stream}", handlers.Stream)
	mux.HandleFunc("GET /Videos/{id}/{stream...}", handlers.Stream)
	mux.HandleFunc("GET /Audio/{id}/{stream}", handlers.Stream)
	mux.HandleFunc("GET /Videos/{id}/{source}/Subtitles/{index}/{stream}", handlers.Subtitle)
	mux.HandleFunc("POST /Sessions/Playing", handlers.Progress)
	mux.HandleFunc("POST /Sessions/Playing/Progress", handlers.Progress)
	mux.HandleFunc("POST /Sessions/Playing/Stopped", handlers.Progress)
	mux.HandleFunc("GET /UserItems/{id}/UserData", handlers.UserData)
	mux.HandleFunc("POST /UserItems/{id}/UserData", handlers.UserData)
	mux.HandleFunc("POST /Users/{user}/PlayedItems/{id}", handlers.Played)
	mux.HandleFunc("DELETE /Users/{user}/PlayedItems/{id}", handlers.Played)
	mux.HandleFunc("POST /UserPlayedItems/{id}", handlers.Played)
	mux.HandleFunc("DELETE /UserPlayedItems/{id}", handlers.Played)
	mux.HandleFunc("POST /UserFavoriteItems/{id}", handlers.Favorite)
	mux.HandleFunc("DELETE /UserFavoriteItems/{id}", handlers.Favorite)
	return nil
}
