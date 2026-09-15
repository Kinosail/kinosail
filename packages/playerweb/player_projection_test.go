package playerweb

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func TestMediaProjectionAndResume(t *testing.T) {
	data := PlayerData{Start: 5, CanFetchSubtitles: true, SubtitleLanguage: "fr"}
	media := mediaprobe.Result{Summary: "Video", Duration: 90, Bitrate: 42, Audio: []mediaprobe.AudioTrack{{}}, Chapters: []mediaprobe.Chapter{{}}, Markers: []mediaprobe.Marker{{}}}
	media.Video.Width, media.Video.Height, media.Video.FrameRate = 1280, 720, 24
	media.ReplayGain.Track, media.ReplayGain.TrackSet = -2.5, true
	data.ApplyMedia(media)
	want := PlayerData{
		Start: 5, CanFetchSubtitles: true, SubtitleLanguage: "fr",
		MediaDetails: "Video", Duration: 90, MediaBitrate: 42,
		MediaWidth: 1280, MediaHeight: 720, MediaFrameRate: 24,
		Audio: []mediaprobe.AudioTrack{{}}, Chapters: []mediaprobe.Chapter{{}}, Markers: []mediaprobe.Marker{{}},
		ReplayGainTrack: "-2.5",
	}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("media projection = %#v", data)
	}
	for _, test := range []struct {
		start, duration float64
		want            bool
	}{{0, 90, false}, {5, 0, true}, {5, 90, true}, {80, 90, false}, {90, 90, false}} {
		data.Start, data.Duration = test.start, test.duration
		if data.Resume() != test.want {
			t.Fatalf("resume(%v, %v)", test.start, test.duration)
		}
	}
}

func TestFinalizeSelectsDeferredSourceAndAddsSession(t *testing.T) {
	for _, test := range []struct {
		agent, adaptive, mime string
		deferSource           bool
	}{{"iPhone", "/hls/item/index.m3u8", "video/x-matroska", true}, {"iPad", "/hls/item/index.m3u8", "video/mp4", false}, {"Macintosh Safari/", "/hls/item/index.m3u8", "video/x-matroska", true}, {"Macintosh Safari/ Chrome/", "/hls/item/index.m3u8", "video/x-matroska", false}, {"Macintosh", "/hls/item/index.m3u8", "video/x-matroska", false}, {"iPhone", "", "video/x-matroska", false}, {"Firefox", "/hls/item/index.m3u8", "video/x-matroska", false}} {
		data := PlayerData{Source: "/media/item?audio=2", DirectSource: "/media/item", AdaptiveSource: test.adaptive, DirectType: test.mime, PlaybackSession: "session"}
		if test.agent == "iPad" {
			data.Plan.Mode, data.Plan.Reason = "transcode", "codec"
		}
		request := httptest.NewRequestWithContext(t.Context(), "GET", "/watch/item", nil)
		request.Header.Set("User-Agent", test.agent)
		data.Finalize(request)
		if data.DeferDirect != test.deferSource || data.Source != "/media/item?audio=2&playbackSession=session" || data.DirectSource != "/media/item?playbackSession=session" {
			t.Fatalf("finalize = %#v", data)
		}
		if test.adaptive != "" && data.AdaptiveSource != test.adaptive+"?playbackSession=session" {
			t.Fatalf("adaptive = %s", data.AdaptiveSource)
		}
	}
	if SessionURL("%zz", "s") != "%zz" {
		t.Fatal("malformed source changed")
	}
}

func TestPlayerTemplateEnhancesSharedControls(t *testing.T) {
	source := PlayerTemplate(`<audio aria-label="{{.Title}}" data-title="{{.Title}}"><video aria-label="{{.Title}}" data-auto-skip="{{.AutoSkip}}"><span data-player-message>Loading video…</span><button type="button" aria-label="Enter fullscreen" data-player-fullscreen><button class="quiet" type="button" data-cast>Play on device</button><script src="/static/player.js?v=34"></script>`)
	for _, want := range []string{"data-playback-session", "data-home-assistant", "data-artist", "data-playback-policy", "data-player-fallback hidden>Try again", "data-player-pip", "/static/player.js?v=34"} {
		if !strings.Contains(source, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(source, `data-cast>Play on device`) {
		t.Fatal("obsolete cast control remains")
	}
}
