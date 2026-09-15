package jellyfincompat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
)

func playbackFixture(item library.Item, effects *[]string) PlaybackHTTP {
	return PlaybackHTTP{
		VisibleItem: func(_ *http.Request, id string) (library.Item, bool) {
			*effects = append(*effects, "visible")
			return item, id == item.ID
		},
		VisibleLibrary: func(*http.Request) ([]library.Item, error) { return []library.Item{item}, nil },
		PrepareImageRequest: func(request *http.Request) *http.Request {
			*effects = append(*effects, "prepare")
			return request
		},
		Inspect: func(context.Context, library.Item) PlaybackMedia {
			*effects = append(*effects, "inspect")
			return PlaybackMedia{Facts: playback.MediaFacts{Kind: item.Kind, Container: "mp4", Duration: 60, Video: playback.VideoFacts{Codec: "h264"}}, Markers: []markers.Marker{{Type: "intro", End: 5}}}
		},
		PreferredVideoCodec: func([]string) string { return "h264" },
		ViewerPolicy: func(*http.Request) playback.ViewerPolicy {
			return playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
		},
		Decide: func(_ playback.MediaFacts, _ playback.ClientCapabilities, policy playback.ViewerPolicy, _ playback.NetworkIntent, _ []markers.Marker, _ []string) playback.PlaybackPlan {
			*effects = append(*effects, "decide")
			return playback.PlaybackPlan{Allowed: true, Mode: map[bool]string{true: "direct", false: "blocked"}[policy.AllowTranscode], SubtitleMode: "none", ColorMode: "preserve"}
		},
		AutomaticSkip: func() []string { return []string{"intro"} },
		NewSession: func(*http.Request, string, playback.PlaybackPlan) (string, error) {
			*effects = append(*effects, "session")
			return "play", nil
		},
		SessionToken: func(*http.Request) (string, string) { return "token", "x-emby-token" },
		SourcePolicy: MediaSourcePolicy{IncludeToken: true, QueryOnDirectPath: true, TranscodingProtocolField: "TranscodingSubProtocol"},
		Resolved:     func(context.Context, library.Item) { *effects = append(*effects, "resolved") },
	}
}

func playbackRequest(t *testing.T, method, id, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "/Items/value/PlaybackInfo", strings.NewReader(body))
	request.SetPathValue("id", id)
	return request
}

func TestPlaybackHTTPRegistersAndRunsPlayerPlanningOrder(t *testing.T) { //nolint:cyclop // One flow verifies the complete shared orchestration.
	t.Parallel()
	item := library.Item{ID: "item", Kind: "video", Title: "Film"}
	effects := make([]string, 0)
	module := playbackFixture(item, &effects)
	handler := func(http.ResponseWriter, *http.Request) {}
	mux := http.NewServeMux()
	if err := module.Register(mux, playback.JellyfinPlaybackHandlers{Stream: handler, Subtitle: handler, Progress: handler, UserData: handler, Played: handler, Favorite: handler}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := playbackRequest(t, http.MethodPost, item.ID, `{"DeviceProfile":{"TranscodingProfiles":[]}}`)
	module.PlaybackInfo(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"PlaySessionId":"play"`) || !strings.Contains(response.Body.String(), "api_key=token") || strings.Join(effects, ",") != "visible,resolved,inspect,decide,session" {
		t.Fatalf("playback = %d %q effects=%v", response.Code, response.Body.String(), effects)
	}
	effects = effects[:0]
	profileBody := `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video"}]}}`
	module.PlaybackInfo(httptest.NewRecorder(), playbackRequest(t, http.MethodPost, item.ID, profileBody))
	if strings.Join(effects, ",") != "visible,resolved,inspect,decide,session" {
		t.Fatalf("profile flow effects = %v", effects)
	}
	if err := module.Register(nil, playback.JellyfinPlaybackHandlers{}); err == nil {
		t.Fatal("invalid registration succeeded")
	}
}

func TestPlaybackHTTPRejectsInvalidInputsBeforeExpensiveWork(t *testing.T) { //nolint:cyclop // The matrix proves no probe or session side effect.
	t.Parallel()
	item := library.Item{ID: "item", Kind: "video"}
	for name, request := range map[string]*http.Request{
		"missing id":   playbackRequest(t, http.MethodPost, "", `{}`),
		"control id":   playbackRequest(t, http.MethodPost, "bad\n", `{}`),
		"oversized id": playbackRequest(t, http.MethodPost, strings.Repeat("x", 129), `{}`),
		"unknown id":   playbackRequest(t, http.MethodPost, "other", `{}`),
	} {
		effects := make([]string, 0)
		response := httptest.NewRecorder()
		playbackFixture(item, &effects).PlaybackInfo(response, request)
		if response.Code != http.StatusNotFound || strings.Contains(strings.Join(effects, ","), "inspect") {
			t.Fatalf("%s = %d effects=%v", name, response.Code, effects)
		}
	}
	effects := make([]string, 0)
	module := playbackFixture(item, &effects)
	invalid := httptest.NewRecorder()
	module.PlaybackInfo(invalid, playbackRequest(t, http.MethodPost, item.ID, `{}{}`))
	if invalid.Code != http.StatusBadRequest || strings.Join(effects, ",") != "visible" {
		t.Fatalf("invalid body = %d effects=%v", invalid.Code, effects)
	}
	effects = effects[:0]
	module.SessionToken = func(*http.Request) (string, string) { return strings.Repeat("x", 257), "x-emby-token" }
	badToken := httptest.NewRecorder()
	module.PlaybackInfo(badToken, playbackRequest(t, http.MethodPost, item.ID, `{}`))
	if badToken.Code != http.StatusBadRequest || strings.Join(effects, ",") != "visible" {
		t.Fatalf("invalid token = %d effects=%v", badToken.Code, effects)
	}
	photoEffects := make([]string, 0)
	photo := playbackFixture(library.Item{ID: "photo", Kind: "photo"}, &photoEffects)
	photoResponse := httptest.NewRecorder()
	photo.PlaybackInfo(photoResponse, playbackRequest(t, http.MethodGet, "photo", ""))
	if photoResponse.Code != http.StatusNotFound {
		t.Fatalf("photo = %d", photoResponse.Code)
	}
}

func TestPlaybackHTTPMapsSessionAndSourceFailures(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "item", Kind: "video"}
	for name, configure := range map[string]func(*PlaybackHTTP){
		"session": func(module *PlaybackHTTP) {
			module.NewSession = func(*http.Request, string, playback.PlaybackPlan) (string, error) { return "", errors.New("persist") }
		},
		"source": func(module *PlaybackHTTP) {
			module.NewSession = func(*http.Request, string, playback.PlaybackPlan) (string, error) {
				return strings.Repeat("x", 257), nil
			}
		},
	} {
		effects := make([]string, 0)
		module := playbackFixture(item, &effects)
		configure(&module)
		response := httptest.NewRecorder()
		module.PlaybackInfo(response, playbackRequest(t, http.MethodGet, item.ID, ""))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s failure = %d %q", name, response.Code, response.Body.String())
		}
	}
	if _, err := MediaSource(item, playback.MediaFacts{}, playback.PlaybackPlan{}, MediaSourcePolicy{}, "", strings.Repeat("x", 257)); err == nil {
		t.Fatal("oversized source token was accepted")
	}
	effects := make([]string, 0)
	module := playbackFixture(item, &effects)
	module.SessionToken = func(*http.Request) (string, string) { return "cookie", "kinosail-session-cookie" }
	response := httptest.NewRecorder()
	module.PlaybackInfo(response, playbackRequest(t, http.MethodGet, item.ID, ""))
	if strings.Contains(response.Body.String(), "cookie") {
		t.Fatal("browser cookie was projected")
	}
	module.SourcePolicy.IncludeToken = false
	module.SessionToken = nil
	module.PlaybackInfo(httptest.NewRecorder(), playbackRequest(t, http.MethodGet, item.ID, ""))
}

func TestPlaybackHTTPSelectsAndServesArtwork(t *testing.T) { //nolint:cyclop // One artwork matrix protects every media-kind selection and failure response.
	t.Parallel()
	root := t.TempDir()
	art := filepath.Join(root, "art.jpg")
	if err := os.WriteFile(art, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	direct := library.Item{ID: "direct", Kind: "video", Artwork: art}
	effects := make([]string, 0)
	module := playbackFixture(direct, &effects)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items/direct/Images/Primary", nil)
	request.SetPathValue("id", "direct")
	response := httptest.NewRecorder()
	module.Image(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "image" || !strings.Contains(strings.Join(effects, ","), "prepare") {
		t.Fatalf("image = %d %q effects=%v", response.Code, response.Body.String(), effects)
	}
	for _, id := range []string{"", strings.Repeat("x", 129), "missing"} {
		candidate := playbackRequest(t, http.MethodGet, id, "")
		missing := httptest.NewRecorder()
		module.Image(missing, candidate)
		if missing.Code != http.StatusNotFound {
			t.Fatalf("missing image %q = %d", id, missing.Code)
		}
	}

	episode := library.Item{ID: "episode", Kind: "video", Show: "Series", Season: 1, Episode: 1, Artwork: art, ShowArtwork: art}
	secondary := playbackFixture(library.Item{}, &effects)
	secondary.VisibleItem = func(*http.Request, string) (library.Item, bool) { return library.Item{}, false }
	secondary.VisibleLibrary = func(*http.Request) ([]library.Item, error) { return []library.Item{episode}, nil }
	showID := ID(ShowID("Series"))
	if item, found := secondary.ArtworkItem(request, showID); !found || item.Artwork != art {
		t.Fatalf("show artwork = %#v, %t", item, found)
	}
	if item, found := secondary.ArtworkItem(request, SeasonID(ShowID("Series"), 1)); !found || item.Artwork != art {
		t.Fatalf("season artwork = %#v, %t", item, found)
	}
	if _, found := secondary.ArtworkItem(request, "missing"); found {
		t.Fatal("unknown artwork was found")
	}
}
