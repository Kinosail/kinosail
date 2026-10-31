package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestDTSSurroundReachesEAC3ClientsAsFiveOne(t *testing.T) {
	ffmpeg, ffprobe := hevcTools(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	media, cache, data := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Surround Movie.mkv")
	commands := [][]string{}
	artifacts := map[string][]byte{}
	run := func(name, executable string, arguments ...string) []byte {
		t.Helper()
		return hevcCommand(t, ctx, &commands, artifacts, name, executable, arguments...)
	}
	run("generate", ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24000/1001:duration=6",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=6",
		"-filter_complex", "[1:a]pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0[a]", "-map", "0:v", "-map", "[a]",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "24", "-c:a", "dca", "-strict", "-2", source)
	host := httptest.NewServer(server.New(server.Config{Lifecycle: ctx, MediaDir: media, DataDir: data, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}))
	defer host.Close()
	request := func(method, path string, body any, status int) []byte {
		t.Helper()
		return hevcRequest(t, ctx, host, method, path, body, status)
	}
	var listing struct{ Items []struct{ ID string } }
	if err := json.Unmarshal(request(http.MethodGet, "/api/v1/library", nil, http.StatusOK), &listing); err != nil || len(listing.Items) != 1 {
		t.Fatalf("library: %v, %d items", err, len(listing.Items))
	}
	item := "/api/v1/items/" + listing.Items[0].ID
	plan := surroundPlan(t, request, item+"/playback?videoCodecs=h264&audioCodecs=aac,ac3,eac3&maxAudioChannels=6")
	if plan.CompatiblePlan.Mode != "audio-transcode" || plan.CompatiblePlan.AudioCodec != "eac3" {
		t.Fatalf("E-AC-3 client plan = %+v", plan.CompatiblePlan)
	}
	awaitHEVCPreparation(t, request, item, plan.Compatible, artifacts)
	master := string(request(http.MethodGet, plan.Compatible, nil, http.StatusOK))
	if !strings.Contains(master, "ec-3") {
		t.Fatalf("master playlist does not announce E-AC-3: %s", master)
	}
	probe := strings.Fields(string(run("probe", ffprobe, "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_name,channels", "-of", "csv=p=0", host.URL+plan.Compatible)))
	if len(probe) == 0 || strings.Trim(probe[0], ",") != "eac3,6" {
		t.Fatalf("delivered audio = %q, want eac3,6", probe)
	}
	stereo := surroundPlan(t, request, item+"/playback?videoCodecs=h264&audioCodecs=aac&maxAudioChannels=2")
	if stereo.CompatiblePlan.AudioCodec != "aac" {
		t.Fatalf("stereo client plan = %+v", stereo.CompatiblePlan)
	}
}

type surroundPlayback struct {
	Compatible     string
	CompatiblePlan struct{ Mode, AudioCodec string }
}

func surroundPlan(t *testing.T, request func(string, string, any, int) []byte, path string) surroundPlayback {
	t.Helper()
	var plan surroundPlayback
	if err := json.Unmarshal(request(http.MethodGet, path, nil, http.StatusOK), &plan); err != nil {
		t.Fatal(err)
	}
	return plan
}
