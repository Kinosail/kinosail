package jellyfincompat

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
)

type deliveryTestSession struct {
	item, profile string
	revision      uint64
	expires       time.Time
	public        bool
	plan          playback.PlaybackPlan
}

func (session deliveryTestSession) JellyfinItemID() string { return session.item }
func (session deliveryTestSession) JellyfinProfile() (string, uint64) {
	return session.profile, session.revision
}
func (session deliveryTestSession) JellyfinExpires() time.Time { return session.expires }
func (session deliveryTestSession) JellyfinPublic() bool       { return session.public }
func (session deliveryTestSession) JellyfinPlan() playback.PlaybackPlan {
	return session.plan
}

type deliveryState struct {
	item              library.Item
	viewer, current   DeliveryViewer
	session           any
	now               time.Time
	hls               []string
	rejected          int
	visible           bool
	allowed, canView  bool
	canDownload       bool
	downloads         bool
	startErr, waitErr error
	download          downloads.Job
	embedded          int
	extractPath       string
	extractData       []byte
	extractErr        error
	subtitle          string
	timeline          *playback.Timeline
	unavailable       int
	media             PlaybackMedia
	plan              playback.PlaybackPlan
}

func newDeliveryState(t *testing.T) *deliveryState {
	t.Helper()
	root := t.TempDir()
	media := filepath.Join(root, "film.mp4")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	state := &deliveryState{
		item:   library.Item{ID: "item000000000000", Kind: "video", Path: media, Subtitles: []string{"film.vtt"}},
		viewer: DeliveryViewer{ID: "viewer", Revision: 2, Owner: true, CanTranscode: true},
		now:    now, visible: true, allowed: true, canView: true, canDownload: true,
		media: PlaybackMedia{Facts: playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 60, Video: playback.VideoFacts{Codec: "h264"}}},
		plan:  playback.PlaybackPlan{Allowed: true, Mode: "direct", SubtitleMode: "none", ColorMode: "preserve"},
	}
	state.current = state.viewer
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: now.Add(time.Hour), plan: state.plan}
	return state
}

func (state *deliveryState) module() DeliveryHTTP {
	playbackModule := PlaybackHTTP{
		VisibleItem: func(_ *http.Request, id string) (library.Item, bool) {
			return state.item, state.visible && id == state.item.ID
		},
		Inspect: func(context.Context, library.Item) PlaybackMedia { return state.media },
		ViewerPolicy: func(*http.Request) playback.ViewerPolicy {
			return playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
		},
		Decide: func(playback.MediaFacts, playback.ClientCapabilities, playback.ViewerPolicy, playback.NetworkIntent, []markers.Marker, []string) playback.PlaybackPlan {
			return state.plan
		},
		AutomaticSkip: func() []string { return []string{"intro"} },
	}
	return DeliveryHTTP{
		Playback:      playbackModule,
		Policy:        DeliveryPolicy{HLS: playback.HLSRecipePolicy{MaxBitrate: 1_000_000_000_000, OffsetStepMilliseconds: 100}, SourceHLS: true},
		CurrentViewer: func(*http.Request) DeliveryViewer { return state.current },
		FindItem:      func(id string) (library.Item, bool) { return state.item, id == state.item.ID },
		FindViewer: func(id string) (DeliveryViewer, bool) {
			return state.viewer, id == state.viewer.ID
		},
		ViewerAllowed: func(*http.Request, string, time.Time) bool { return state.allowed },
		CanView:       func(string, library.Item) bool { return state.canView },
		Public:        func(request *http.Request) bool { return request.Header.Get("X-Public") == "true" },
		LoadSession:   func(id string) (any, bool) { return state.session, id == "play" && state.session != nil },
		ActiveRevision: func(id string, revision uint64) bool {
			return id == state.viewer.ID && revision == state.viewer.Revision
		},
		Now:      func() time.Time { return state.now },
		Rejected: func(context.Context, PlaybackRejection) { state.rejected++ },
		ServeHLS: func(_ http.ResponseWriter, _ *http.Request, _ library.Item, _ playback.HLSRecipe, file string) {
			state.hls = append(state.hls, file)
		},
		HLSConfigured: func() bool { return true }, CanDownload: func(*http.Request) bool { return state.canDownload },
		DownloadsConfigured: func() bool { return state.downloads },
		StartDownload: func(string, library.Item) (downloads.Job, error) {
			return state.download, state.startErr
		},
		WaitDownload: func(context.Context, string, string) (downloads.Job, error) {
			return state.download, state.waitErr
		},
		WriteEmbedded: func(http.ResponseWriter, *http.Request, library.Item, int) { state.embedded++ },
		ExtractEmbedded: func(context.Context, library.Item, int) (string, []byte, error) {
			return state.extractPath, state.extractData, state.extractErr
		},
		WriteSubtitle: func(_ http.ResponseWriter, _ *http.Request, path string, timeline *playback.Timeline) {
			state.subtitle, state.timeline = path, timeline
		},
		SubtitleUnavailable: func(context.Context, library.Item, int, error) { state.unavailable++ },
	}
}

func deliveryRequest(t *testing.T, stream, query string) *http.Request {
	t.Helper()
	target := "/Videos/item/" + stream
	if query != "" {
		target += "?" + query
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.SetPathValue("id", ID("item000000000000"))
	request.SetPathValue("stream", stream)
	return request
}

func TestDeliverySessionAndContextEnforceEveryBinding(t *testing.T) { //nolint:cyclop,funlen,gocognit // One fixture varies each session boundary.
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	request := deliveryRequest(t, "stream", "playSessionId=play")
	if session, valid := module.PlaySession(request, state.item.ID); !valid || session.JellyfinItemID() != state.item.ID {
		t.Fatalf("play session = %#v, %t", session, valid)
	}
	for _, query := range []string{"", "playSessionId=" + strings.Repeat("x", 257), "playSessionId=play&PlaySessionId=play"} {
		invalid := deliveryRequest(t, "stream", query)
		if _, valid := module.PlaySession(invalid, state.item.ID); valid {
			t.Fatalf("invalid query %q was authorized", query)
		}
	}
	state.session = "invalid"
	if _, valid := module.PlaySession(request, state.item.ID); valid {
		t.Fatal("invalid session type was authorized")
	}
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(time.Hour), plan: state.plan}
	if item, viewer, found := module.PlaybackContext(request); !found || item.ID != state.item.ID || viewer.ID != "viewer" {
		t.Fatalf("authenticated context = %#v, %#v, %t", item, viewer, found)
	}
	if _, _, found := module.PlaybackContext(deliveryRequest(t, "stream", "playSessionId=play&PlaySessionId=play")); found {
		t.Fatal("conflicting authenticated session query was accepted")
	}
	state.current = DeliveryViewer{}
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(time.Hour), public: true, plan: state.plan}
	public := deliveryRequest(t, "stream", "playSessionId=play")
	public.Header.Set("X-Public", "true")
	if item, viewer, found := module.PlaybackContext(public); !found || item.ID != state.item.ID || viewer.ID != "viewer" {
		t.Fatalf("public context = %#v, %#v, %t", item, viewer, found)
	}
	if _, _, found := module.PlaybackContext(deliveryRequest(t, "stream", "")); found {
		t.Fatal("public context without a session was accepted")
	}
	state.session = deliveryTestSession{item: "other", profile: "viewer", revision: 2, expires: state.now.Add(time.Hour), public: true, plan: state.plan}
	if _, _, found := module.PlaybackContext(public); found || state.rejected != 1 {
		t.Fatalf("mismatched context found=%t rejected=%d", found, state.rejected)
	}
	for _, id := range []string{"", strings.Repeat("x", 129)} {
		invalid := deliveryRequest(t, "stream", "playSessionId=play")
		invalid.SetPathValue("id", id)
		if _, _, found := module.PlaybackContext(invalid); found {
			t.Fatalf("invalid item id %q was accepted", id)
		}
	}
	state.current = DeliveryViewer{ID: "viewer"}
	state.visible = false
	if _, _, found := module.PlaybackContext(request); found {
		t.Fatal("hidden authenticated item was accepted")
	}
	state.now = time.Now()
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", expires: time.Now().Add(time.Minute), plan: state.plan}
	module = state.module()
	module.Now = nil
	if _, valid := module.PlaySession(request, state.item.ID); !valid {
		t.Fatal("default clock rejected a current session")
	}
}

func TestDeliveryHelpersPreservePlayerHLSForms(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	public := func(request *http.Request) bool { return request.Header.Get("X-Public") == "true" }
	recipe := playback.HLSRecipe{Mode: "transcode"}
	if local := HLSRecipe(request, recipe, true, public); !local.SingleQuality {
		t.Fatal("local Player recipe is not single quality")
	}
	request.Header.Set("X-Public", "true")
	if remote := HLSRecipe(request, recipe, true, public); remote.SingleQuality {
		t.Fatal("public Player recipe is single quality")
	}
	if unchanged := HLSRecipe(request, recipe, false, nil); unchanged.SingleQuality {
		t.Fatal("Subtitles recipe changed")
	}
	for path, want := range map[string]string{
		"main/1080p/index.m3u8": "1080p/index.m3u8",
		"main/index.m3u8":       "index.m3u8",
		"main/master.m3u8":      "index.m3u8",
	} {
		if file, valid := SourceHLSFile(path); !valid || file != want {
			t.Fatalf("source HLS %q = %q, %t", path, file, valid)
		}
	}
	for _, path := range []string{"index.m3u8", "other/file.txt"} {
		if _, valid := SourceHLSFile(path); valid {
			t.Fatalf("invalid source HLS %q was accepted", path)
		}
	}
}

func TestDeliveryDownloadMapsAllOutcomes(t *testing.T) { //nolint:cyclop // Each I/O outcome has a distinct response.
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	request := deliveryRequest(t, "stream", "")
	if module.serveDownload(httptest.NewRecorder(), request, state.item, state.viewer) {
		t.Fatal("non-download route was handled")
	}
	request.URL.Path = "/Items/item/Download"
	state.canDownload = false
	forbidden := httptest.NewRecorder()
	if !module.serveDownload(forbidden, request, state.item, state.viewer) || forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden download = %d", forbidden.Code)
	}
	state.canDownload, state.downloads = true, false
	if module.serveDownload(httptest.NewRecorder(), request, state.item, state.viewer) {
		t.Fatal("unconfigured download was handled")
	}
	state.downloads, state.startErr = true, errors.New("start")
	unavailable := httptest.NewRecorder()
	if !module.serveDownload(unavailable, request, state.item, state.viewer) || unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed download = %d", unavailable.Code)
	}
	state.startErr, state.waitErr = nil, errors.New("wait")
	if response := httptest.NewRecorder(); !module.serveDownload(response, request, state.item, state.viewer) || response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed wait = %d", response.Code)
	}
	state.waitErr, state.download = nil, downloads.Job{ID: "job", File: filepath.Join(t.TempDir(), "missing")}
	missing := httptest.NewRecorder()
	if !module.serveDownload(missing, request, state.item, state.viewer) || missing.Code != http.StatusNotFound {
		t.Fatalf("missing file = %d", missing.Code)
	}
	file := filepath.Join(t.TempDir(), "download.mp4")
	if err := os.WriteFile(file, []byte("download"), 0o600); err != nil {
		t.Fatal(err)
	}
	state.download = downloads.Job{ID: "job", File: file, Title: "Film", Size: int64(len("download")), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("download")))}
	success := httptest.NewRecorder()
	if !module.serveDownload(success, request, state.item, state.viewer) || success.Code != http.StatusOK || success.Body.String() != "download" {
		t.Fatalf("download = %d %q", success.Code, success.Body.String())
	}
}
