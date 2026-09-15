package playback

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestHTTPPlaybackInputsAreStrictAndSideEffectFree(t *testing.T) {
	t.Parallel()
	for query, expected := range map[string][]string{"": nil, "?videoCodecs=h264,hevc": {"h264", "hevc"}} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+query, nil)
		got, err := RequestedVideoCodecs(request)
		if err != nil || !reflect.DeepEqual(got, expected) {
			t.Fatalf("query %q = %#v, %v", query, got, err)
		}
	}
	for _, query := range []string{"?videoCodecs=", "?videoCodecs=H264", "?videoCodecs=h264,h264", "?videoCodecs=h264,hevc,av1,vp9,h264", "?videoCodecs=unknown", "?videoCodecs=h264&videoCodecs=hevc"} {
		if _, err := RequestedVideoCodecs(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+query, nil)); err == nil {
			t.Errorf("accepted %q", query)
		}
	}
	if SubtitleRoleLabel("") != "Subtitles" || SubtitleRoleLabel("captions") != "Captions" {
		t.Fatal("subtitle role labels changed")
	}
}

func TestTraceReaderRejectsBeforeCallerSideEffects(t *testing.T) {
	t.Parallel()
	validSession := func(value string) bool { return value == "trace-session" }
	valid := `{"session":"trace-session","event":"play","sequence":1,"method":"direct","detail":"control:ok"}`
	response := httptest.NewRecorder()
	event, err := ReadTrace(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(valid)), validSession)
	if err != nil || event.Event != "play" || !ValidTrace(event, validSession) || !SafeTraceText(event.Detail) {
		t.Fatalf("trace = %#v, %v", event, err)
	}
	for _, body := range []string{
		`{"session":"trace-session","event":"play","sequence":0}`,
		`{"session":"trace-session","event":"unknown","sequence":1}`,
		`{"session":"trace-session","event":"play","sequence":1,"unknown":true}`,
		valid + `{}`,
		`{"session":"trace-session","event":"play","sequence":1,"readyState":5}`,
		`{"session":"trace-session","event":"play","sequence":1,"detail":"bad value"}`,
	} {
		if _, err := ReadTrace(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)), validSession); err == nil {
			t.Errorf("accepted %q", body)
		}
	}
	if _, err := ReadTrace(nil, nil, nil); err == nil || ValidTrace(TraceEvent{}, nil) || SafeTraceText(strings.Repeat("x", 65)) {
		t.Fatal("accepted missing or oversized trace input")
	}
}

func TestJellyfinPlaybackRequestAndCapabilities(t *testing.T) { //nolint:cyclop // Strict wire input and its capability result share one contract.
	t.Parallel()
	body := `{"DeviceProfile":{"DirectPlayProfiles":[{"Container":"matroska,mp4","Type":"Video","VideoCodec":"h264,hevc","AudioCodec":"aac"}],"DirectStreamProfiles":[{"Container":"mp4"}],"TranscodingProfiles":[{"VideoCodec":"hevc,h264"}],"SubtitleProfiles":[{"Format":"vtt","Method":"External"}]},"MaxStreamingBitrate":4000000}`
	input, provided, valid := ReadJellyfinPlaybackRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)))
	if !provided || !valid || input.MaxStreamingBitrate != 4_000_000 {
		t.Fatalf("playback request = %#v, %v, %v", input, provided, valid)
	}
	facts := MediaFacts{Kind: "video", Video: VideoFacts{Codec: "hevc", HDR: "hdr10"}}
	capabilities := JellyfinCapabilities(input.DeviceProfile, facts)
	if !Includes(capabilities.Containers, "mkv") || !Includes(capabilities.VideoCodecs, "hevc") || !Includes(capabilities.AudioCodecs, "aac") || !capabilities.SupportsRemux || !capabilities.SupportsExternalSubtitles || Includes(capabilities.HDRFormats, "hdr10") || !Includes(capabilities.HDRFormats, "sdr") {
		t.Fatalf("capabilities = %#v", capabilities)
	}
	if input, provided, valid := ReadJellyfinPlaybackRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)); !valid || provided || !reflect.DeepEqual(input, JellyfinPlaybackRequest{}) {
		t.Fatalf("GET input = %#v, %v, %v", input, provided, valid)
	}
	invalidBodies := []string{`{"MaxStreamingBitrate":-1}`, `{"MaxStreamingBitrate":1000000000001}`, `{"AudioStreamIndex":1025}`, `{"DeviceProfile":{"DirectPlayProfiles":[` + strings.Repeat(`{},`, 129) + `{ }]}}`, `{"DeviceProfile":{"SubtitleProfiles":[{"Format":"` + strings.Repeat("x", 1025) + `"}]}}`, `{invalid`}
	for _, invalid := range invalidBodies {
		if _, _, ok := ReadJellyfinPlaybackRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(invalid))); ok {
			t.Errorf("accepted invalid Jellyfin request length %d", len(invalid))
		}
	}
	oversized := strings.Repeat(" ", maximumJellyfinRequestBytes+1)
	if _, _, ok := ReadJellyfinPlaybackRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(oversized))); ok {
		t.Fatal("accepted oversized Jellyfin request")
	}
}

func TestJellyfinMediaSourcePreservesBothProductContracts(t *testing.T) { //nolint:cyclop // Player and Subtitles wire compatibility is compared together.
	t.Parallel()
	item := library.Item{ID: "0123456789abcdef", Title: "Film", Size: 42, Subtitles: []string{"film.en.srt"}}
	facts := MediaFacts{Container: "mkv", Duration: 100, Video: VideoFacts{Codec: "h264", Profile: "high", Level: "4.1", Width: 1920, Height: 1080, BitDepth: 8}, Audio: []AudioFacts{{Index: 2, SourceIndex: 4, Codec: "aac", Default: true}}, Subtitles: []SubtitleFacts{{SourceIndex: 6, Codec: "vtt", Text: true}}}
	plan := PlaybackPlan{Allowed: true, Mode: "transcode", VideoCodec: "h264", AudioIndex: 2, SubtitleMode: "none", ColorMode: "preserve", MarkerMode: "server", Timeline: Timeline{Duration: 80}}
	player, err := JellyfinMediaSource(item, facts, plan, JellyfinMediaSourceOptions{PlaySessionID: "play", Token: "token", QueryOnDirectPath: true, TranscodingProtocolField: "TranscodingSubProtocol"})
	if err != nil || !strings.Contains(player["Path"].(string), "api_key=token") || player["TranscodingSubProtocol"] != "hls" || player["RunTimeTicks"] != int64(800_000_000) || player["DefaultAudioStreamIndex"] != 4 {
		t.Fatalf("Player media source = %#v, %v", player, err)
	}
	subtitles, err := JellyfinMediaSource(item, facts, plan, JellyfinMediaSourceOptions{PlaySessionID: "play", TranscodingProtocolField: "TranscodingProtocol"})
	if err != nil || strings.Contains(subtitles["Path"].(string), "?") || subtitles["TranscodingProtocol"] != "hls" {
		t.Fatalf("Subtitles media source = %#v, %v", subtitles, err)
	}
	if streams := JellyfinMediaStreams(item, facts, "?x=1"); len(streams) != 3 {
		t.Fatalf("streams = %#v", streams)
	}
	if _, err := JellyfinMediaSource(item, facts, plan, JellyfinMediaSourceOptions{TranscodingProtocolField: "bad"}); err == nil {
		t.Fatal("accepted unknown protocol field")
	}
	direct, err := JellyfinMediaSource(item, facts, PlaybackPlan{Allowed: true, Mode: "direct"}, JellyfinMediaSourceOptions{TranscodingProtocolField: "TranscodingProtocol"})
	if err != nil || direct["SupportsDirectPlay"] != true || direct["SupportsTranscoding"] != false {
		t.Fatalf("direct source = %#v, %v", direct, err)
	}
}

func TestJellyfinRoutesSessionsAndBitrate(t *testing.T) { //nolint:cyclop // Related Jellyfin protocol adapters share one focused test.
	called := 0
	handler := func(http.ResponseWriter, *http.Request) { called++ }
	mux := http.NewServeMux()
	all := JellyfinPlaybackHandlers{handler, handler, handler, handler, handler, handler, handler, handler}
	if err := RegisterJellyfinPlayback(mux, all); err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items/id/PlaybackInfo", nil), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Sessions/Playing", nil), httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/UserPlayedItems/id", nil)} {
		mux.ServeHTTP(httptest.NewRecorder(), request)
	}
	if called != 3 {
		t.Fatalf("route calls = %d", called)
	}
	if err := RegisterJellyfinPlayback(nil, JellyfinPlaybackHandlers{}); err == nil {
		t.Fatal("accepted missing handlers")
	}

	now := time.Now()
	store := &sync.Map{}
	store.Store("expired", testJellyfinSession{item: "old", profile: "owner", expires: now.Add(-time.Second)})
	session := testJellyfinSession{item: "item", profile: "owner", revision: 2, expires: now.Add(time.Hour), public: true}
	id, err := StoreJellyfinPlaySession(store, now, func() (string, error) { return "session", nil }, session)
	if err != nil || id != "session" {
		t.Fatalf("stored session = %q, %v", id, err)
	}
	if _, found := store.Load("expired"); found {
		t.Fatal("expired session was not pruned")
	}
	value, found := store.Load("session")
	allowed := AuthorizeJellyfinPlaySession(value, found, "item", "owner", true, now, func(id string, revision uint64) bool { return id == "owner" && revision == 2 })
	if !allowed || AuthorizeJellyfinPlaySession(value, found, "other", "owner", true, now, func(string, uint64) bool { return true }) || AuthorizeJellyfinPlaySession(nil, false, "item", "owner", false, now, nil) {
		t.Fatal("session authorization changed")
	}
	if _, err := StoreJellyfinPlaySession(store, now, func() (string, error) { return "", errors.New("entropy") }, session); err == nil {
		t.Fatal("ignored session ID failure")
	}

	for path, status := range map[string]int{"/Playback/BitrateTest": http.StatusOK, "/Playback/BitrateTest?Size=5": http.StatusOK, "/Playback/BitrateTest?size=0": http.StatusBadRequest, "/Playback/BitrateTest?bad=1": http.StatusBadRequest, "/Playback/BitrateTest?size=1&Size=2": http.StatusBadRequest} {
		response := httptest.NewRecorder()
		JellyfinBitrateTest(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != status {
			t.Errorf("%s = %d", path, response.Code)
		}
	}
	if _, ok := JellyfinBitrateTestSize(nil); ok {
		t.Fatal("accepted nil bitrate request")
	}
}

type testJellyfinSession struct {
	item, profile string
	revision      uint64
	expires       time.Time
	public        bool
}

func (session testJellyfinSession) JellyfinItemID() string { return session.item }
func (session testJellyfinSession) JellyfinProfile() (string, uint64) {
	return session.profile, session.revision
}
func (session testJellyfinSession) JellyfinExpires() time.Time { return session.expires }
func (session testJellyfinSession) JellyfinPublic() bool       { return session.public }

func TestSubtitlesAndTrickplay(t *testing.T) { //nolint:cyclop // Presentation-time helpers are tested together.
	t.Parallel()
	timeline := Timeline{SourceDuration: 20, Duration: 10, Omitted: []Range{{Start: 5, End: 15}}}
	input := []byte("WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nkeep\n\n00:00:06.000 --> 00:00:08.000\ndrop\n\n00:00:16.000 --> 00:00:18.000 line:0\nshift")
	output := string(MapWebVTT(input, timeline))
	if !strings.Contains(output, "00:00:01.000 --> 00:00:03.000") || strings.Contains(output, "drop") || !strings.Contains(output, "00:00:06.000 --> 00:00:08.000 line:0") {
		t.Fatalf("mapped VTT = %q", output)
	}
	if seconds, err := ParseVTTTime("01:02:03.500"); err != nil || seconds != 3723.5 || FormatVTTTime(seconds) != "01:02:03.500" {
		t.Fatalf("VTT time = %v, %v", seconds, err)
	}
	if _, err := ParseVTTTime("bad"); err == nil {
		t.Fatal("accepted invalid VTT time")
	}

	if _, err := NewTrickplay(TrickplayDependencies{}); err == nil {
		t.Fatal("accepted missing trickplay dependencies")
	}
	root := t.TempDir()
	target := filepath.Join(root, "frame.jpg")
	fake := filepath.Join(root, "ffmpeg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nfor target do :; done\nprintf frame > \"$target\"\n"), 0o700); err != nil { //nolint:gosec // Test fixture must be executable.
		t.Fatal(err)
	}
	if err := GenerateTrickplay(t.Context(), root, fake, filepath.Join(root, "source.mkv"), target, 10); err != nil || !pathExists(target) {
		t.Fatalf("trickplay generation = %v", err)
	}
	if err := GenerateTrickplay(t.Context(), "", fake, "source", target, 0); err == nil {
		t.Fatal("accepted unconfigured trickplay")
	}
}
