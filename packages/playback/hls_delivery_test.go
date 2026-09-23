package playback

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestHLSDeliveryValidatesBeforeCallbacks(t *testing.T) { //nolint:cyclop,gocognit,funlen // Route success and negative no-side-effect cases share one contract.
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	selection, err := ParseHLSRequest("p/t-a0-s0-none-t0-b0/360p/index.m3u8", "1", policy)
	if err != nil || !selection.Planned || selection.Track != 1 || selection.File != "360p/index.m3u8" {
		t.Fatalf("selection = %#v, %v", selection, err)
	}
	for _, input := range [][2]string{{"p/bad/index.m3u8", "0"}, {"../secret", "0"}, {"index.m3u8", "32"}} {
		if _, err := ParseHLSRequest(input[0], input[1], policy); err == nil {
			t.Errorf("accepted route %#v", input)
		}
	}
	for _, test := range []struct {
		mode string
		want string
	}{
		{"remux", "-c:v copy"}, {"audio-transcode", "-c:a aac"}, {"transcode", "-maxrate 872k"},
	} {
		arguments, err := HLSCodecArguments(HLSCodecInput{Arguments: []string{"ffmpeg"}, Video: []string{"-vf", "scale", "-c:v", "libx264"}, Compatibility: []string{"-profile:v", "high"}, ItemPath: "movie.mkv", VideoRate: "2000k", AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: test.mode, Audio: 1, MaxBitrate: 1_000_000}, Policy: policy})
		if err != nil || !strings.Contains(strings.Join(arguments, " "), test.want) {
			t.Fatalf("%s arguments = %#v, %v", test.mode, arguments, err)
		}
	}
	if _, err := HLSCodecArguments(HLSCodecInput{}); err == nil {
		t.Fatal("accepted invalid codec input")
	}
	effected, err := HLSCodecArguments(HLSCodecInput{Arguments: []string{"ffmpeg"}, Video: []string{"-c:v", "copy"}, VideoRate: "2000k", AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: "audio-transcode", DialogueBoost: true, NormalizeLoudness: true}, Policy: policy})
	if err != nil || !strings.Contains(strings.Join(effected, " "), "-af equalizer=f=1600") || !strings.Contains(strings.Join(effected, " "), "dynaudnorm=") {
		t.Fatalf("audio enhancement arguments = %#v, %v", effected, err)
	}
	if _, err := HLSCodecArguments(HLSCodecInput{AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: "remux", DialogueBoost: true}, Policy: policy}); err == nil {
		t.Fatal("remux accepted an audio effect")
	}

	item := library.Item{ID: "0123456789abcdef", Kind: "video"}
	served, notFound, forbidden, invalidSeek := 0, 0, 0, 0
	dependencies := HLSHandlerDependencies{
		Policy:      policy,
		Lookup:      func(*http.Request, string) (library.Item, bool) { return item, true },
		Allowed:     func(*http.Request) bool { return true },
		Legacy:      func(*http.Request, library.Item, int) HLSRecipe { return HLSRecipe{Mode: "transcode", Codec: "hevc"} },
		Duration:    func(*http.Request, library.Item) float64 { return 100 },
		Serve:       func(http.ResponseWriter, *http.Request, library.Item, HLSRecipe, string) { served++ },
		NotFound:    func(http.ResponseWriter, *http.Request) { notFound++ },
		Forbidden:   func(http.ResponseWriter, *http.Request) { forbidden++ },
		InvalidSeek: func(http.ResponseWriter, *http.Request) { invalidSeek++ },
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/id/index.m3u8", nil)
	request.SetPathValue("id", item.ID)
	request.SetPathValue("file", "index.m3u8")
	ServeHLS(httptest.NewRecorder(), request, dependencies)
	if served != 1 {
		t.Fatal("legacy HLS was not served")
	}
	dependencies.Allowed = func(*http.Request) bool { return false }
	ServeHLS(httptest.NewRecorder(), request, dependencies)
	if forbidden != 1 || served != 1 {
		t.Fatal("forbidden HLS caused a side effect")
	}
	dependencies.Allowed = func(*http.Request) bool { return true }
	request.SetPathValue("file", "p/t-a0-s0-none-t0-b0-o200000/360p/index.m3u8")
	ServeHLS(httptest.NewRecorder(), request, dependencies)
	if invalidSeek != 1 || served != 1 {
		t.Fatal("invalid seek caused a side effect")
	}
	request.SetPathValue("file", "p/bad/index.m3u8")
	ServeHLS(httptest.NewRecorder(), request, dependencies)
	if notFound != 1 || served != 1 {
		t.Fatal("invalid route caused a side effect")
	}
	ServeHLS(httptest.NewRecorder(), nil, HLSHandlerDependencies{})
}

func TestTrickplayHandlerRejectsInvalidInputWithoutGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	callback := ""
	frames, err := NewTrickplay(TrickplayDependencies{
		Cache: root, FFmpeg: "/missing", RecipePolicy: HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100},
		Lookup: func(*http.Request, string) (library.Item, bool) {
			return library.Item{ID: "0123456789abcdef", Kind: "video", Path: "/missing"}, true
		},
		NotFound:    func(http.ResponseWriter, *http.Request) { callback = "not-found" },
		Unavailable: func(http.ResponseWriter, *http.Request) { callback = "unavailable" },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trickplay/id/bad", nil)
	request.SetPathValue("id", "0123456789abcdef")
	request.SetPathValue("second", "bad")
	frames.Serve(httptest.NewRecorder(), request)
	if callback != "not-found" {
		t.Fatalf("callback = %q", callback)
	}
	callback = ""
	request.SetPathValue("second", "10")
	request.URL.RawQuery = "playbackToken=bad"
	frames.Serve(httptest.NewRecorder(), request)
	if callback != "not-found" {
		t.Fatalf("token callback = %q", callback)
	}
}
