package jellyfincompat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

func TestDeliveryStreamPreservesPlayerSelectionOrder(t *testing.T) { //nolint:cyclop // Each route form selects one delivery mode.
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	direct := httptest.NewRecorder()
	module.Stream(direct, deliveryRequest(t, "stream", ""))
	if direct.Code != http.StatusOK || direct.Body.String() != "video" {
		t.Fatalf("direct stream = %d %q", direct.Code, direct.Body.String())
	}
	module.Policy.NestedStream = true
	nested := httptest.NewRecorder()
	module.Stream(nested, deliveryRequest(t, "source/stream.mp4", ""))
	if nested.Code != http.StatusOK || nested.Body.String() != "video" {
		t.Fatalf("nested stream = %d %q", nested.Code, nested.Body.String())
	}
	state.plan.MarkerMode = "server"
	selected := httptest.NewRecorder()
	module.Stream(selected, deliveryRequest(t, "stream", ""))
	if len(state.hls) != 1 {
		t.Fatalf("automatic delivery was not selected: %v", state.hls)
	}
	for _, stream := range []string{"other", strings.Repeat("x", 2049)} {
		response := httptest.NewRecorder()
		module.Stream(response, deliveryRequest(t, stream, ""))
		if response.Code != http.StatusNotFound {
			t.Fatalf("invalid stream %d bytes = %d", len(stream), response.Code)
		}
	}
	state.visible = false
	hidden := httptest.NewRecorder()
	module.Stream(hidden, deliveryRequest(t, "stream", ""))
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("hidden stream = %d", hidden.Code)
	}
	state.visible, state.item.Kind = true, "photo"
	photo := httptest.NewRecorder()
	module.Stream(photo, deliveryRequest(t, "stream", ""))
	if photo.Code != http.StatusNotFound {
		t.Fatalf("photo stream = %d", photo.Code)
	}
}

func TestDeliveryExplicitHLSValidatesBeforeServing(t *testing.T) { //nolint:cyclop // The table protects explicit Player HLS forms.
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	state.current.Owner, state.current.CanTranscode = false, false
	forbidden := httptest.NewRecorder()
	module.Stream(forbidden, deliveryRequest(t, "master.m3u8", ""))
	if forbidden.Code != http.StatusForbidden || len(state.hls) != 0 {
		t.Fatalf("forbidden HLS = %d served=%v", forbidden.Code, state.hls)
	}
	state.current.Owner = true
	for _, query := range []string{"", "playbackPlan=bad", "playbackPlan=bad&PlaybackPlan=bad"} {
		response := httptest.NewRecorder()
		module.Stream(response, deliveryRequest(t, "master.m3u8", query))
		if response.Code != http.StatusNotFound || len(state.hls) != 0 {
			t.Fatalf("invalid plan %q = %d served=%v", query, response.Code, state.hls)
		}
	}
	recipe := playback.HLSRecipe{Mode: "transcode", Codec: "h264", Audio: 0, Subtitle: 0, MaxBitrate: 4_000_000}
	token := recipe.Token()
	master := httptest.NewRecorder()
	module.Stream(master, deliveryRequest(t, "master.m3u8", "playbackPlan="+token))
	if len(state.hls) != 1 || state.hls[0] != "index.m3u8" {
		t.Fatalf("master HLS = %v", state.hls)
	}
	planned := httptest.NewRecorder()
	module.Stream(planned, deliveryRequest(t, "p/"+token+"/1080p/index.m3u8", ""))
	if len(state.hls) != 2 || state.hls[1] != "1080p/index.m3u8" {
		t.Fatalf("planned HLS = %v", state.hls)
	}
	state.current.Owner = false
	deniedPlanned := httptest.NewRecorder()
	module.Stream(deniedPlanned, deliveryRequest(t, "p/"+token+"/index.m3u8", ""))
	if deniedPlanned.Code != http.StatusForbidden {
		t.Fatalf("denied planned HLS = %d", deniedPlanned.Code)
	}
}

func TestDeliverySessionHLSAndAutomaticSkipUseAppPolicy(t *testing.T) { //nolint:cyclop // Player and Subtitles choose distinct session policies.
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	request := deliveryRequest(t, "index.m3u8", "playSessionId=play")
	state.plan.Mode = "transcode"
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: state.plan}
	module.Policy.RequireHLSCache = true
	module.HLSConfigured = func() bool { return false }
	if module.serveSessionHLS(httptest.NewRecorder(), request, state.item) {
		t.Fatal("uncached Player HLS was selected")
	}
	module.HLSConfigured = func() bool { return true }
	if !module.serveSessionHLS(httptest.NewRecorder(), request, state.item) {
		t.Fatal("cached Player HLS was not selected")
	}
	state.plan.MarkerMode = "server"
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: state.plan}
	module.Policy.RequireHLSCache, module.Policy.SessionTimelineOnly = false, true
	if !module.serveSessionHLS(httptest.NewRecorder(), request, state.item) {
		t.Fatal("Subtitles timeline HLS was not selected")
	}
	state.session = nil
	if module.serveSessionHLS(httptest.NewRecorder(), request, state.item) {
		t.Fatal("missing session selected HLS")
	}
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: state.plan}
	state.item.Kind = "photo"
	if module.serveAutomaticSkip(httptest.NewRecorder(), request, state.item, state.current) {
		t.Fatal("photo automatic skip was selected")
	}
	state.item.Kind = "video"
	viewer := DeliveryViewer{}
	if module.serveAutomaticSkip(httptest.NewRecorder(), request, state.item, viewer) {
		t.Fatal("unprivileged automatic skip was selected")
	}
	state.plan.MarkerMode = ""
	if module.serveAutomaticSkip(httptest.NewRecorder(), request, state.item, state.current) {
		t.Fatal("unmapped automatic skip was selected")
	}
	state.plan.MarkerMode = "server"
	if !module.serveAutomaticSkip(httptest.NewRecorder(), request, state.item, state.current) {
		t.Fatal("automatic skip HLS was not selected")
	}
}

func TestDeliverySourceHLSRequiresMatchingSession(t *testing.T) {
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	state.plan.Mode = "transcode"
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: state.plan}
	served := httptest.NewRecorder()
	module.Stream(served, deliveryRequest(t, "main/1080p/index.m3u8", "playSessionId=play"))
	if len(state.hls) != 1 || state.hls[0] != "1080p/index.m3u8" {
		t.Fatalf("source HLS = %v", state.hls)
	}
	state.hls = nil
	missing := httptest.NewRecorder()
	module.Stream(missing, deliveryRequest(t, "index.m3u8", ""))
	if missing.Code != http.StatusNotFound || len(state.hls) != 0 {
		t.Fatalf("missing session HLS = %d served=%v", missing.Code, state.hls)
	}
}
