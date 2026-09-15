package playerweb

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (data *PlayerData) ApplyMedia(media mediaprobe.Result) {
	data.MediaDetails, data.Audio = media.Summary, media.Audio
	data.MediaBitrate, data.MediaWidth, data.MediaHeight, data.MediaFrameRate = media.Bitrate, media.Video.Width, media.Video.Height, media.Video.FrameRate
	data.Duration, data.Chapters, data.Markers = media.Duration, media.Chapters, media.Markers
	data.ReplayGainTrack = FormatReplayGain(media.ReplayGain.Track, media.ReplayGain.TrackSet)
	data.ReplayGainAlbum = FormatReplayGain(media.ReplayGain.Album, media.ReplayGain.AlbumSet)
}

func (data *PlayerData) Finalize(request *http.Request) {
	data.DeferDirect = data.AdaptiveSource != "" && AppleWebKitClient(request.UserAgent()) && strings.Contains(data.DirectType, "matroska")
	data.Source, data.DirectSource, data.AdaptiveSource = SessionURL(data.Source, data.PlaybackSession), SessionURL(data.DirectSource, data.PlaybackSession), SessionURL(data.AdaptiveSource, data.PlaybackSession)
	method, reason := data.Plan.Mode, data.Plan.Reason
	if method == "" {
		method, reason = "direct", "original-media"
	}
	slog.InfoContext(request.Context(), "playback planned", "playback_session", data.PlaybackSession, "kind", data.Kind, "policy", data.PlaybackPolicy, "method", method, "reason", reason, "compatibility_method", data.CompatibilityMode, "transcode_allowed", data.CanTranscode, "override", data.PlaybackOverride, "container", data.Plan.Container, "video_codec", data.Plan.VideoCodec, "audio_codec", data.Plan.AudioCodec, "width", data.Plan.Width, "height", data.Plan.Height, "direct_type", data.DirectType)
}

func AppleWebKitClient(userAgent string) bool {
	return strings.Contains(userAgent, "iPhone") || strings.Contains(userAgent, "iPad") || strings.Contains(userAgent, "Macintosh") && strings.Contains(userAgent, "Safari/") && !strings.Contains(userAgent, "Chrome/")
}

func SessionURL(source, session string) string {
	if source == "" {
		return ""
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return source
	}
	query := parsed.Query()
	query.Set("playbackSession", session)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func FormatReplayGain(gain float64, present bool) string {
	if !present {
		return ""
	}
	return strconv.FormatFloat(gain, 'f', -1, 64)
}
