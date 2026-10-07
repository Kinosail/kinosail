package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// Audio encoding has a complete final AAC packet beyond the source's nominal
// duration. Every fragment advertised by the public completed stream must be
// retrievable, including direct cached requests after a Server restart.
func TestRealAudioHLSHTTPCompletedFragmentsSurviveRestart(t *testing.T) {
	for _, value := range []struct{ extension, codec, kind string }{{".flac", "flac", "audio"}, {".m4b", "alac", "audiobook"}} {
		t.Run(value.kind, func(t *testing.T) { assertCompleteAudioHLS(t, value.extension, value.codec) })
	}
}

func assertCompleteAudioHLS(t *testing.T, extension, codec string) {
	t.Helper()
	f, ffmpeg := realAudioHLSFixture(t, extension, codec)
	handler, ctx, config := f.handler, t.Context(), f.config
	source := f.source
	master := speedTestGET(t, ctx, handler, source)
	variants := speedTestURIs(master)
	if len(variants) != 1 {
		t.Fatalf("audio renditions=%d", len(variants))
	}
	base := source[:strings.LastIndex(source, "/")+1] + strings.TrimSuffix(variants[0], "index.m3u8")
	playlist := speedTestGET(t, ctx, handler, base+"index.m3u8")
	if !bytes.Contains(playlist, []byte("#EXT-X-ENDLIST")) {
		t.Fatal("audio fixture must be completed before cached adoption")
	}
	segments := speedTestURIs(playlist)
	if len(segments) < 5 || len(segments) > 7 {
		t.Fatalf("audio fragments=%d", len(segments))
	}
	initialization := speedTestGET(t, ctx, handler, base+"init.mp4")
	complete := bytes.Clone(initialization)
	for _, segment := range segments {
		complete = append(complete, speedTestGET(t, ctx, handler, base+segment)...)
	}
	restart, cancelRestart := context.WithCancel(t.Context())
	defer cancelRestart()
	config.Lifecycle = restart
	restarted := server.New(config)
	for _, segment := range segments {
		original := speedTestGET(t, t.Context(), handler, base+segment)
		if !bytes.Equal(original, speedTestGET(t, t.Context(), restarted, base+segment)) {
			t.Fatal("restart changed cached audio media")
		}
	}
	assertAudioDeliveredSamples(t, ffmpeg, complete)
}

func realAudioHLSFixture(t *testing.T, extension, codec string) (copiedHTTPFixture, string) {
	t.Helper()
	ffmpeg, ffprobe := copiedHLSTools(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	media, cache := t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Fixture"+extension)
	generate := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=10", "-c:a", codec, "-ac", "2", source) //nolint:gosec // Fixed synthetic audio and discovered local tool.
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate audio fixture: %v: %s", err, output)
	}
	tools := t.TempDir()
	starts, response, probes := filepath.Join(tools, "encodes"), filepath.Join(tools, "response"), filepath.Join(tools, "probes")
	if err := os.WriteFile(starts, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	encoder, probe := filepath.Join(tools, "encoder"), filepath.Join(tools, "probe")
	quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
	servertest.WriteExecutable(t, encoder, "#!/bin/sh\nprintf encode\\n >> "+quote(starts)+"\nexec "+quote(ffmpeg)+" \"$@\"\n")
	actual := filepath.Join(tools, "actual-packet-metadata")
	t.Cleanup(func() {
		if t.Failed() {
			if b, err := os.ReadFile(actual); err == nil && len(b) < 4096 {
				t.Logf("emitted AAC final metadata: %s", b)
			}
		}
	})
	servertest.WriteExecutable(t, probe, "#!/bin/sh\ncase \" $* \" in\n*\" stream=codec_name,profile,sample_rate:\"*) printf probe\\n >> "+quote(probes)+"; if [ -f "+quote(response)+" ]; then cat >/dev/null; cat "+quote(response)+"; exit 0; fi; "+quote(ffprobe)+" \"$@\" > "+quote(actual)+"; status=$?; cat "+quote(actual)+"; exit $status;;\nesac\nexec "+quote(ffprobe)+" \"$@\"\n")
	logs := captureCopiedLogs(t)
	retainCopiedFailureFacts(t, ffmpeg, ffprobe, source, cache, logs)
	retainCopiedPlaylistFacts(t, cache)
	config := server.Config{Lifecycle: ctx, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFmpeg: encoder, FFprobe: probe}
	handler, id := formatTestItem(t, config)
	var plan struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &plan)
	if plan.CompatiblePlan.Mode != "audio-transcode" {
		t.Fatalf("audio plan=%s", plan.CompatiblePlan.Mode)
	}
	assertAPIBody(t, prepareSource(t, handler, id, plan.Compatible), http.StatusAccepted)
	// Actual playback adopts queued preparation before its speculative window is cancelled.
	speedTestGET(t, ctx, handler, plan.Compatible)
	awaitCopiedLog(t, logs, "HLS transcode completed", "", "")
	cancel()
	config.Lifecycle = t.Context()
	handler = server.New(config)
	directory := audioRecipeDirectory(t, cache, id)

	return copiedHTTPFixture{handler: handler, config: config, output: logs, id: id, source: plan.Compatible, directory: directory, starts: starts, endpoint: response, probes: probes}, ffmpeg
}

// These controls exercise untrusted probe responses over genuinely encoded AAC
// media. A fabricated endpoint must not authorize a cached terminal fragment.
func TestRealAudioHLSHTTPTerminalMetadataRejectsWithoutMutation(t *testing.T) {
	base, _ := realAudioHLSFixture(t, ".flac", "flac")
	speedTestGET(t, t.Context(), base.handler, base.source)
	for _, value := range []struct{ name, output, asset string }{
		{"missing", `{}`, ""},
		{"malformed", `not-json`, ""},
		{"oversized", strings.Repeat("x", (1<<20)+1), ""},
		{"unknown response field", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"unexpected":"do-not-record","frames":[{"pts_time":"10.005333","nb_samples":768}]}`, ""},
		{"unknown profile", `{"streams":[{"codec_name":"aac","profile":"HE-AAC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":768}]}`, ""},
		{"missing decoded samples", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333"}]}`, ""},
		{"unknown codec", `{"streams":[{"codec_name":"opus","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":1024}]}`, ""},
		{"missing sample rate", `{"streams":[{"codec_name":"aac","profile":"LC"}],"frames":[{"pts_time":"10.005333","nb_samples":1024}]}`, ""},
		{"out of range rate", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"1"}],"frames":[{"pts_time":"10.005333","nb_samples":49152000}]}`, ""},
		{"conflicting streams", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"},{"codec_name":"aac","profile":"LC","sample_rate":"44100"}],"frames":[{"pts_time":"10.005333","nb_samples":1024}]}`, ""},
		{"duplicate rate", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"1"}],"frames":[{"pts_time":"10.005333","nb_samples":1024}]}`, ""},
		{"nonfinite clock", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"Infinity","nb_samples":1024}]}`, ""},
		{"beyond AAC padding", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"11","nb_samples":1024}]}`, ""},
		{"invalid decoded samples", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":4800}]}`, ""},
		{"missing frame clock", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"nb_samples":1024}]}`, ""},
		{"zero samples", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":0}]}`, ""},
		{"fractional samples", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":1023.5}]}`, ""},
		{"duplicate samples", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":1024,"nb_samples":1}]}`, ""},
		{"excessive frames", `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[` + strings.Repeat(`{"pts_time":"10.005333","nb_samples":1024},`, 4096) + `{"pts_time":"10.005333","nb_samples":1024}]}`, ""},
		{"truncated initialization", "", "init.mp4"},
		{"unknown cached ordinal", "", "segment-99999.m4s"},
		{"oversized manifest", "", "index.m3u8"},
	} {
		t.Run(value.name, func(t *testing.T) {
			assertRejectedAudioEnd(t, cloneCompletedAudioCache(t, base), value.output, value.asset)
		})
	}
}

func assertRejectedAudioEnd(t *testing.T, f copiedHTTPFixture, output, asset string) {
	t.Helper()
	name := damageAudioTerminalFixture(t, f, output, asset)
	before := snapshotCopiedPolicyCache(t, f, true)
	sourceBefore, err := os.ReadFile(filepath.Join(f.config.MediaDir, "Fixture.flac"))
	if err != nil {
		t.Fatal(err)
	}
	logBefore := len(f.output.snapshot())
	probesBefore, _ := os.ReadFile(f.probes)
	response := apiCall(t, server.New(f.config), "", http.MethodGet, strings.TrimSuffix(f.source, "index.m3u8")+"audio/"+name, nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("terminal metadata HTTP status=%d, expected404", response.Code)
	}
	if output != "" {
		assertAudioTerminalFailureLog(t, f.output.snapshot()[logBefore:])
		probesAfter, err := os.ReadFile(f.probes)
		if err != nil || len(probesAfter) <= len(probesBefore) {
			t.Fatal("terminal metadata control did not reach the emitted-packet probe")
		}
	}
	sourceAfter, err := os.ReadFile(filepath.Join(f.config.MediaDir, "Fixture.flac"))
	if err != nil || !bytes.Equal(sourceBefore, sourceAfter) {
		t.Fatal("rejected AAC metadata changed its source media")
	}
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
		t.Fatal("invalid AAC endpoint mutated cache or encoded media")
	}
}

func cloneCompletedAudioCache(t *testing.T, original copiedHTTPFixture) copiedHTTPFixture {
	t.Helper()
	f := original
	f.config.CacheDir = t.TempDir()
	f.config.Lifecycle = t.Context()
	f.config.DataDir = t.TempDir()
	f.directory = filepath.Join(f.config.CacheDir, filepath.Base(original.directory))
	if err := os.CopyFS(f.directory, os.DirFS(original.directory)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.endpoint); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	f.handler = server.New(f.config)
	return f
}

func audioCacheDamage(t *testing.T, f copiedHTTPFixture, asset string) []byte {
	t.Helper()
	if asset == "index.m3u8" {
		manifest, err := os.ReadFile(filepath.Join(f.directory, "audio", asset))
		if err != nil {
			t.Fatal(err)
		}
		return append(manifest, append([]byte("\n#"), bytes.Repeat([]byte("x"), (1<<20)+1)...)...)
	}
	return []byte("invalid owned cache asset")
}

func assertAudioDeliveredSamples(t *testing.T, ffmpeg string, complete []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "delivered.mp4")
	if err := os.WriteFile(path, complete, 0o600); err != nil {
		t.Fatal(err)
	}
	frames := copiedDecodedFrames(t, t.Context(), ffmpeg, path, "0:a:0")
	samples := 0
	for _, frame := range frames {
		samples += frame.samples / 2
	} // Two independently generated source channels.
	t.Logf("audio delivered: decoded frames=%d samples/channel=%d duration=%.6f", len(frames), samples, float64(samples)/48000)
	// 480000 source samples round to 469 LC frames, plus one encoder delay
	// frame: 470 complete decoded frames and 481280 samples per channel.
	if len(frames) != 470 || samples != 481280 {
		t.Fatal("AAC decoded sample count changed its source rounding and encoder delay")
	}

	for i := 1; i < len(frames); i++ {
		expected := float64(frames[i-1].samples/2) / 48000
		delta := frames[i].time - frames[i-1].time
		if math.Abs(delta-expected) > 1.0/48000 {
			t.Fatalf("AAC decoded gap at frame %d: %.6f", i, delta)
		}
	}
}

func audioRecipeDirectory(t *testing.T, cache, id string) string {
	t.Helper()
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), id) {
			return filepath.Join(cache, entry.Name())
		}
	}
	t.Fatal("audio recipe cache absent")
	return ""
}

func assertAudioTerminalFailureLog(t *testing.T, log string) {
	t.Helper()
	if strings.Contains(log, "do-not-record") {
		t.Fatal("terminal metadata appeared in operational log")
	}
	for _, line := range strings.Split(log, "\n") {
		var value map[string]any
		if json.Unmarshal([]byte(line), &value) != nil || value["msg"] != "HLS audio terminal media rejected" {
			continue
		}
		id, ok := value["request_id"].(string)
		if !ok || id == "" || len(id) > 128 || value["level"] != "WARN" || value["failure_class"] != "invalid-terminal-metadata" {
			t.Fatal("terminal media rejection lacks safe correlated warning")
		}
		return
	}
	t.Fatal("terminal media rejection warning absent")
}

func damageAudioTerminalFixture(t *testing.T, f copiedHTTPFixture, output, asset string) string {
	t.Helper()
	if output != "" {
		if err := os.WriteFile(f.endpoint, []byte(output), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	name := "segment-00005.m4s"
	if asset != "" {
		if err := writeCopiedCacheFile(f.config.CacheDir, filepath.Join(f.directory, "audio", asset), audioCacheDamage(t, f, asset)); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(asset, "segment-") {
			name = asset
		}
	}

	return name
}
