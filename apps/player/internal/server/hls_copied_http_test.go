package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

// Registered HTTP operations own these tests. External process fixtures isolate
// malformed demux/IDR/configuration responses and missing cache fragments, which
// the real-codec decode suite cannot reliably generate. Failures to guard: unsafe
// timeline admission, guessed segment cuts, speculative encodes after rejection,
// stale cache reuse, refill clock drift, and missing full-duration manifests.
const copiedPackets = "time_base=1/1000\npts=0|dts=0|duration=4000|flags=K_\npts=4000|dts=4000|duration=4000|flags=K_\npts=8000|dts=8000|duration=4000|flags=K_\n"

var copiedIDRs = "#tb 0: 1/1000\n0, 0, 0, 4000, 128, " + strings.Repeat("a", 64) + "\n0, 4000, 4000, 4000, 128, " + strings.Repeat("b", 64) + "\n0, 8000, 8000, 4000, 128, " + strings.Repeat("c", 64) + "\n"
var copiedConfiguration = "#format: frame checksums\n0, 0, 0, 4000, 64, " + strings.Repeat("d", 64) + "\n"

type copiedHTTPFixture struct {
	handler                             http.Handler
	output                              *copiedLogBuffer
	release                             string
	config                              server.Config
	id, source, directory, starts, done string
	clock, endpoint, probes             string
}

func newCopiedHTTPFixture(t *testing.T, packets, idrs, configuration string) copiedHTTPFixture {
	t.Helper()
	media, tools, cache := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Copied.mp4"), []byte("owned source"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe, encoder := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg")
	starts, done := filepath.Join(tools, "encodes"), filepath.Join(tools, "scan-done")
	release := filepath.Join(tools, "release")
	clock, endpoint, probes := filepath.Join(tools, "clock.json"), filepath.Join(tools, "endpoint.json"), filepath.Join(tools, "metadata-probes")
	for path, value := range map[string]string{clock: `{"packets":[{"pts_time":"0.083","flags":"K_"}]}`, endpoint: `{"packets":[{"pts_time":"12.043","duration_time":"0.04"}]}`} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	servertest.WriteExecutable(t, probe, "#!/bin/sh\ncase \" $* \" in\n"+
		"*\" compact=p=0 \"*) printf '%s' "+quote(packets)+"; printf done > "+quote(done)+";;\n"+
		"*\" packet=pts_time,flags \"*) cat >/dev/null; printf 'clock\\n' >> "+quote(probes)+"; cat "+quote(clock)+";;\n"+
		"*\" packet=pts_time,duration_time \"*) cat >/dev/null; printf 'endpoint\\n' >> "+quote(probes)+"; cat "+quote(endpoint)+";;\n"+
		"*) printf '%s' '{\"streams\":[{\"index\":0,\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":640,\"height\":360},{\"index\":1,\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"12\"}}';;\nesac\n")
	servertest.WriteExecutable(t, encoder, "#!/bin/sh\ncase \" $* \" in\n"+
		"*\" filter_units=pass_types=5 \"*) printf '%s' "+quote(idrs)+"; exit 0;;\n"+
		"*\" h264_mp4toannexb,filter_units=pass_types=7|8 \"*) printf '%s' "+quote(configuration)+"; exit 0;;\nesac\n"+
		"start=0; segments=; previous=\nfor value; do if [ \"$previous\" = -start_number ]; then start=$value; fi; if [ \"$previous\" = -hls_segment_filename ]; then segments=$value; fi; previous=$value; done\n"+
		"for output; do case \"$output\" in */index.m3u8)\nprintf 'encode\\n' >> "+quote(starts)+"\ndirectory=${output%/*}; mediaDirectory=${segments%/*}; mkdir -p \"$directory\" \"$mediaDirectory\"\n"+
		mp4fixture.Shell(mp4fixture.Initialization(640, 360, "h264", "aac", ""))+" > \"$directory/init.mp4\"\n"+
		"segment=$(printf 'segment-%05d.m4s' \"$start\"); printf 'fragment-%s' \"$start\" > \"$mediaDirectory/$segment\"\n"+
		"printf '#EXTM3U\\n#EXT-X-PLAYLIST-TYPE:EVENT\\n#EXT-X-MAP:URI=\"init.mp4\"\\n#EXTINF:4,\\n%s\\n' \"$segment\" > \"$output\"\n"+
		"if [ \"$start\" = 0 ]; then printf fragment-1 > \"$directory/segment-00001.m4s\"; printf '#EXTINF:4,\\nsegment-00001.m4s\\n' >> \"$output\"; printf fragment-2 > \"$directory/segment-00002.m4s\"; printf '#EXTINF:4,\\nsegment-00002.m4s\\n' >> \"$output\"; fi\nprintf '#EXT-X-ENDLIST\\n' >> \"$output\"\nwhile [ ! -f "+quote(release)+" ]; do sleep 0.01; done\n;; esac; done\n")
	config := server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFprobe: probe, FFmpeg: encoder}
	output := captureCopiedLogs(t)
	h, id := firstWebItem(t, config)
	recipe := playback.HLSRecipe{Mode: "remux"}
	return copiedHTTPFixture{handler: h, output: output, release: release, config: config, id: id, source: "/hls/" + id + "/p/r-a0-s0-none-t0-b0/index.m3u8", directory: filepath.Join(cache, playback.HLSRecipeKey(id, recipe)), starts: starts, done: done, clock: clock, endpoint: endpoint, probes: probes}
}

func awaitCopiedReady(t *testing.T, f copiedHTTPFixture) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r := prepareSource(t, f.handler, f.id, f.source)
		assertAPIBody(t, r, http.StatusAccepted)
		if strings.Contains(r.Body.String(), `"state":"ready"`) {
			if f.release != "" {
				awaitCopiedLog(t, f.output, "HLS startup preparation", "", "ready")
				if err := os.WriteFile(f.release, []byte("release"), 0o600); err != nil {
					t.Fatal(err)
				}
				awaitCopiedLog(t, f.output, "HLS transcode paused after playback became inactive", "", "")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("copied startup did not become ready: %s", r.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCopiedHLSHTTPPreparationProjectsAndRefillsCertifiedTimeline(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	master := apiCall(t, f.handler, "", http.MethodGet, f.source, nil)
	assertAPIBody(t, master, http.StatusOK, "360p/index.m3u8")
	rendition := strings.TrimSuffix(f.source, "index.m3u8") + "360p/"
	playlist := apiCall(t, f.handler, "", http.MethodGet, rendition+"index.m3u8", nil)
	assertAPIBody(t, playlist, http.StatusOK, "#EXT-X-PLAYLIST-TYPE:VOD", "#EXTINF:4.000000,", "segment-00002.m4s", "#EXT-X-ENDLIST")
	if strings.Count(playlist.Body.String(), "#EXTINF:") != 3 {
		t.Fatal("full timeline was not delivered")
	}

	before, err := os.ReadFile(f.starts)
	if err != nil {
		t.Fatal(err)
	}
	restarted := server.New(f.config)
	assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, f.source, nil), http.StatusOK, "360p/index.m3u8")
	assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, rendition+"index.m3u8", nil), http.StatusOK, "segment-00002.m4s")
	after, err := os.ReadFile(f.starts)
	if err != nil || string(after) != string(before) {
		t.Fatalf("restart re-encoded reusable copied cache: %q -> %q (%v)", before, after, err)
	}
	// Evicted media retains its certified timeline and is refilled on the public segment request.
	if err := os.Remove(filepath.Join(f.directory, "360p", "segment-00002.m4s")); err != nil {
		t.Fatal(err)
	}
	segment := apiCall(t, restarted, "", http.MethodGet, rendition+"segment-00002.m4s", nil)
	assertAPIBody(t, segment, http.StatusOK, "fragment-2")
	data, err := os.ReadFile(filepath.Join(f.directory, ".copy-timeline"))
	if err != nil {
		t.Fatal(err)
	}
	var certificate struct {
		Clock *float64
		End   float64
	}
	if json.Unmarshal(data, &certificate) != nil || certificate.Clock == nil || certificate.End != 12 {
		t.Fatal("successful startup did not retain its measured timeline")
	}
}

func TestCopiedHLSHTTPPreparationRejectsUncertifiedExternalTimelines(t *testing.T) {
	tests := []struct{ name, packets, idrs, configuration string }{
		{"missing packets", "", copiedIDRs, copiedConfiguration},
		{"malformed time base", strings.Replace(copiedPackets, "1/1000", "0/1000", 1), copiedIDRs, copiedConfiguration},
		{"coarse time base", strings.Replace(copiedPackets, "1/1000", "1/100", 1), copiedIDRs, copiedConfiguration},
		{"negative packet", strings.Replace(copiedPackets, "pts=0", "pts=-1", 1), copiedIDRs, copiedConfiguration},
		{"missing duration", strings.Replace(copiedPackets, "duration=4000", "duration=", 1), copiedIDRs, copiedConfiguration},
		{"missing DTS", strings.Replace(copiedPackets, "dts=4000", "dts=missing", 1), copiedIDRs, copiedConfiguration},
		{"nonincreasing keys", strings.Replace(copiedPackets, "pts=4000", "pts=0", 1), copiedIDRs, copiedConfiguration},
		{"wrong endpoint", strings.Replace(copiedPackets, "pts=8000", "pts=9000", 1), copiedIDRs, copiedConfiguration},
		{"oversized line", strings.Repeat("x", 1025), copiedIDRs, copiedConfiguration},
		{"missing IDRs", copiedPackets, "", copiedConfiguration},
		{"invalid IDR clock", copiedPackets, strings.Replace(copiedIDRs, "1/1000", "1/0", 1), copiedConfiguration},
		{"mismatched IDR", copiedPackets, strings.Replace(copiedIDRs, "0, 4000, 4000", "0, 4000, 5000", 1), copiedConfiguration},
		{"empty IDR packet", copiedPackets, strings.Replace(copiedIDRs, "128", "0", 1), copiedConfiguration},
		{"missing configuration", copiedPackets, copiedIDRs, ""},
		{"malformed configuration", copiedPackets, copiedIDRs, "0, malformed\n"},
		{"short configuration hash", copiedPackets, copiedIDRs, "0, 0, 0, 4000, 64, short\n"},
		{"changed configuration", copiedPackets, copiedIDRs, copiedConfiguration + "0, 4000, 4000, 4000, 64, " + strings.Repeat("e", 64) + "\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newCopiedHTTPFixture(t, test.packets, test.idrs, test.configuration)
			response := prepareSource(t, f.handler, f.id, f.source)
			assertPreparationState(t, response, http.StatusAccepted, "queued")
			awaitCopiedLog(t, f.output, "HLS startup preparation", response.Header().Get("X-Request-ID"), "unavailable")
			t.Cleanup(func() { apiCall(t, f.handler, "", http.MethodDelete, "/api/v1/items/"+f.id+"/playback-prepare", nil) })
			if _, err := os.Stat(f.starts); !os.IsNotExist(err) {
				t.Fatalf("uncertified input started HLS encoding: %v", err)
			}
			if _, err := os.Stat(f.directory); !os.IsNotExist(err) {
				t.Fatalf("uncertified input created media/cache artifacts: %v", err)
			}
		})
	}
}

func TestCopiedHLSHTTPRejectsInvalidPublicSegmentsWithoutRefill(t *testing.T) {
	f := newCopiedHTTPFixture(t, copiedPackets, copiedIDRs, copiedConfiguration)
	awaitCopiedReady(t, f)
	before, err := os.ReadFile(f.starts)
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(f.source, "index.m3u8") + "360p/"
	for _, name := range []string{"segment-00003.m4s", "segment-99999.m4s", "segment--1.m4s", "segment-1.m4s", "unknown.m4s"} {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		r := httptest.NewRecorder()
		f.handler.ServeHTTP(r, httptest.NewRequestWithContext(ctx, http.MethodGet, base+name, nil))
		rejectedBeforeDeadline := ctx.Err() == nil
		cancel()
		if !rejectedBeforeDeadline {
			t.Errorf("invalid segment %q waited for impossible media", name)
		}
		if r.Code != http.StatusNotFound {
			t.Fatalf("invalid segment %q was delivered", name)
		}
	}
	after, err := os.ReadFile(f.starts)
	if err != nil || string(after) != string(before) {
		t.Fatal("invalid public segment consumed encoder admission")
	}
}
