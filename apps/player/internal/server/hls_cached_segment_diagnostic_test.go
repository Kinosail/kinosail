package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// Registered HTTP routes must distinguish cancellation from cached admission
// errors without changing delivery, cache contents, encoding or ID normalization.
// request_context_done describes the observation time, not the error cause.
func TestCachedHLSSegmentFailureDiagnostic(t *testing.T) {
	for _, name := range []string{"valid", "cancelled", "expired", "missing manifest", "changed policy", "outside duration"} {
		t.Run(name, func(t *testing.T) {
			fixture := newCachedSegmentEvidenceFixture(t, "")
			fixture.invalidate(t, name)
			request := fixture.request(t.Context(), fixture.route)
			request = cachedSegmentRequestContext(t, request, name)
			status, events := fixture.deliverWithoutMutation(t, request)
			assertCachedSegmentOutcome(t, name, status, events)
		})
	}
}

func assertCachedSegmentOutcome(t *testing.T, name string, status int, events []map[string]any) {
	t.Helper()
	if name == "valid" {
		if status != http.StatusOK || len(events) != 0 {
			t.Fatal("valid cached delivery reported a failure")
		}
		return
	}
	if status != http.StatusNotFound || len(events) != 1 {
		t.Fatal("cached rejection lost its HTTP status or diagnostic")
	}
	assertCachedSegmentEvent(t, events[0], name == "cancelled" || name == "expired")
	want := map[string]string{"cancelled": "context canceled", "expired": "context deadline exceeded", "missing manifest": "no such file", "changed policy": "playback settings changed", "outside duration": "outside the playable duration"}[name]
	if !strings.Contains(fmt.Sprint(events[0]["error"]), want) {
		t.Fatal("cached rejection did not retain its bounded reason")
	}
}

func cachedSegmentRequestContext(t *testing.T, request *http.Request, name string) *http.Request {
	t.Helper()
	if name == "cancelled" {
		ctx, cancel := context.WithCancel(request.Context())
		cancel()
		return request.WithContext(ctx)
	}
	if name == "expired" {
		ctx, cancel := context.WithDeadline(request.Context(), time.Unix(1, 0))
		t.Cleanup(cancel)
		return request.WithContext(ctx)
	}
	return request
}

func TestCachedHLSSegmentOffsetAndRouteAdmission(t *testing.T) {
	fixture := newCachedSegmentEvidenceFixture(t, "-o1400")
	for _, route := range []string{fixture.route, strings.Replace(fixture.route, fixture.id, "missing-title", 1), strings.Replace(fixture.route, "/p/t-", "/p/unknown-", 1), strings.Replace(fixture.route, "segment-00001.m4s", "unknown.m4s", 1)} {
		status, events := fixture.deliverWithoutMutation(t, fixture.request(t.Context(), route))
		want := http.StatusNotFound
		if route == fixture.route {
			want = http.StatusOK
		}
		if status != want || len(events) != 0 {
			t.Fatal("offset or outer route admission changed cached diagnostics")
		}
	}
}

func TestCachedHLSSegmentDiagnosticMetadataPrivacy(t *testing.T) {
	fixture := newCachedSegmentEvidenceFixture(t, "")
	fixture.invalidate(t, "missing manifest")
	for _, value := range []struct {
		name, query, requestID, session, secondSession, wantSession string
	}{
		{name: "missing"},
		{name: "malformed", requestID: "private request!", session: "private session!"},
		{name: "oversized", requestID: strings.Repeat("private-request-", 80), session: strings.Repeat("private-session-", 80)},
		{name: "query", query: "?playbackSession=query-session", wantSession: "query-session"},
		{name: "conflicting", query: "?playbackSession=query-session", session: "header-session", secondSession: "second-session", wantSession: "header-session"},
		{name: "duplicate query", query: "?playbackSession=first-session&playbackSession=second-session", wantSession: "first-session"},
		{name: "malformed query", query: "?playbackSession=private%20session%21"},
	} {
		t.Run(value.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fixture.route+value.query, nil)
			request.Header.Set("X-Request-ID", value.requestID)
			request.Header.Set("X-Playback-Session", value.session)
			if value.secondSession != "" {
				request.Header.Add("X-Playback-Session", value.secondSession)
			}
			status, events := fixture.deliverWithoutMutation(t, request)
			if status != http.StatusNotFound || len(events) != 1 {
				t.Fatal("metadata changed cached rejection")
			}
			assertCachedSegmentEvent(t, events[0], false)
			if events[0]["playback_session"] != value.wantSession {
				t.Fatal("diagnostic changed existing session normalization")
			}
			if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(fmt.Sprint(events[0]["request_id"])) {
				t.Fatal("diagnostic exposed an invalid request ID")
			}
			assertCachedMetadataPrivacy(t, events[0])
		})
	}
}

type cachedSegmentEvidenceFixture struct {
	handler                                        http.Handler
	id, route, cache, directory, source, arguments string
	logs                                           *copiedLogBuffer
}

func newCachedSegmentEvidenceFixture(t *testing.T, offset string) cachedSegmentEvidenceFixture {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Private Diagnostic Title.mkv")
	if err := os.WriteFile(source, []byte("controlled source"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe, encoder, arguments := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg"), filepath.Join(tools, "arguments")
	writeExecutable(t, probe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":640,\"height\":360},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"duration\":\"12\"}}'\n")
	writeExecutable(t, encoder, fmt.Sprintf("#!/bin/sh\nprintf 'encode\\n' >> '%s'\n", arguments)+servertest.PlayableHLS())
	logs := captureCopiedLogs(t, 64<<10)
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: probe, FFmpeg: encoder})
	token := "t-a0-s0-none-t0-b0" + offset
	recipe, err := playback.ParseHLSRecipe(token, playback.HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100})
	if err != nil {
		t.Fatal(err)
	}
	fixture := cachedSegmentEvidenceFixture{handler: handler, id: id, cache: cache, source: source, arguments: arguments, directory: filepath.Join(cache, playback.HLSRecipeKey(id, recipe)), logs: logs}
	fixture.route = "/hls/" + id + "/p/" + token + "/360p/segment-00001.m4s"
	master := httptest.NewRecorder()
	handler.ServeHTTP(master, fixture.request(t.Context(), "/hls/"+id+"/p/"+token+"/index.m3u8"))
	if master.Code != http.StatusOK {
		t.Fatal("controlled master failed")
	}
	awaitCopiedLog(t, logs, "HLS transcode completed", "cached-diagnostic", "")
	return fixture
}

func (fixture cachedSegmentEvidenceFixture) request(ctx context.Context, route string) *http.Request {
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, route, nil)
	request.Header.Set("X-Request-ID", "cached-diagnostic")
	request.Header.Set("X-Playback-Session", "cached-session")
	return request
}

func (fixture cachedSegmentEvidenceFixture) invalidate(t *testing.T, name string) {
	t.Helper()
	root, err := os.OpenRoot(fixture.directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	const manifest = "360p/index.m3u8"
	switch name {
	case "missing manifest":
		if err := root.Remove(manifest); err != nil {
			t.Fatal(err)
		}
	case "changed policy":
		const master = "index.m3u8"
		data, err := root.ReadFile(master)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.Replace(string(data), "#KINOSAIL-TRANSCODER:", "#KINOSAIL-TRANSCODER:changed-", 1))
		if err := root.WriteFile(master, data, 0o600); err != nil {
			t.Fatal(err)
		}
	case "outside duration":
		data := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:60,\nsegment-00000.m4s\n#EXTINF:4,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n"
		if err := root.WriteFile(manifest, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func (fixture cachedSegmentEvidenceFixture) deliverWithoutMutation(t *testing.T, request *http.Request) (int, []map[string]any) {
	t.Helper()
	before := servertest.SnapshotHLSCache(t, fixture.cache)
	encodedBefore, err := os.ReadFile(fixture.arguments)
	if err != nil {
		t.Fatal(err)
	}
	start := len(fixture.logs.snapshot())
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	encodedAfter, err := os.ReadFile(fixture.arguments)
	if err != nil || string(encodedBefore) != string(encodedAfter) || !reflect.DeepEqual(before, servertest.SnapshotHLSCache(t, fixture.cache)) {
		t.Fatal("cached observation changed encoding or files")
	}
	output := fixture.logs.snapshot()[start:]
	fixture.assertPrivacy(t, output, response.Body.String())
	var events []map[string]any
	for _, line := range strings.Split(output, "\n") {
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) == nil && event["msg"] == "HLS segment preparation failed" && event["cached"] == true {
			events = append(events, event)
		}
	}
	return response.Code, events
}

func (fixture cachedSegmentEvidenceFixture) assertPrivacy(t *testing.T, output, body string) {
	t.Helper()
	for _, private := range []string{fixture.source, filepath.Base(fixture.source), fixture.cache, fixture.directory, fixture.id} {
		if strings.Contains(output, private) || strings.Contains(body, private) {
			t.Fatal("cached observation exposed private source/cache identity")
		}
	}
}

func assertCachedMetadataPrivacy(t *testing.T, event map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private request", "private session", "private-request-", "private-session-", "playbackSession="} {
		if strings.Contains(string(encoded), private) {
			t.Fatal("diagnostic exposed raw metadata")
		}
	}
}

func assertCachedSegmentEvent(t *testing.T, event map[string]any, contextDone bool) {
	t.Helper()
	if event["request_context_done"] != contextDone || event["file"] != "360p/segment-00001.m4s" || event["mode"] != "transcode" || event["diagnostic"] != "[PLAYBACK-HLS]" || len(fmt.Sprint(event["error"])) > 8<<10 {
		t.Fatal("cached rejection diagnostic lost safe bounded fields")
	}
}
