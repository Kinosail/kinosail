package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestShortHEVCStartupPreparationPreservesEveryDecodedVideoFrame(t *testing.T) {
	ffmpeg, ffprobe := hevcTools(t)
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Second)
	defer cancel()
	media, cache, data := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Short HEVC.mkv")
	commands := [][]string{}
	artifacts := map[string][]byte{}
	run := func(name, executable string, arguments ...string) []byte {
		t.Helper()
		return hevcCommand(t, ctx, &commands, artifacts, name, executable, arguments...)
	}
	defer func() { retainHEVCJourney(t, commands, artifacts, source) }()
	run("ffmpeg-version", ffmpeg, "-version")
	run("generate", ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=8.37",
		"-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000:duration=8.37", "-c:v", "libx265", "-preset", "ultrafast",
		"-x265-params", "pools=1:frame-threads=1:keyint=48:min-keyint=48:scenecut=0", "-c:a", "eac3", source)
	original := hevcVideoHashes(run("original-frames", ffmpeg, "-v", "error", "-i", source, "-map", "0:v:0", "-f", "framehash", "-hash", "sha256", "-"))
	if len(original) < 200 {
		t.Fatalf("source has only %d decoded video frames", len(original))
	}
	host := httptest.NewServer(server.New(server.Config{Lifecycle: ctx, MediaDir: media, DataDir: data, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}))
	defer host.Close()
	request := func(method, path string, body any, status int) []byte {
		t.Helper()
		return hevcRequest(t, ctx, host, method, path, body, status)
	}
	item, compatible := hevcPlaybackSource(t, request)
	prepare := func() {
		t.Helper()
		awaitHEVCPreparation(t, request, item, compatible, artifacts)
	}
	prepare()
	for _, phase := range []string{"warm-adoption", "after-cancel", "reprepared"} {
		switch phase {
		case "after-cancel":
			request(http.MethodDelete, item+"/playback-prepare", nil, http.StatusNoContent)
		case "reprepared":
			prepare()
		}
		output := run(phase, ffmpeg, "-v", "error", "-i", host.URL+compatible, "-map", "0:v:0", "-map", "0:a:0", "-f", "framehash", "-hash", "sha256", "-")
		if actual := hevcVideoHashes(output); !reflect.DeepEqual(actual, original) {
			t.Fatalf("%s: %d delivered video frames differ from %d source frames", phase, len(actual), len(original))
		}
		if !strings.Contains(string(output), "#media_type 1: audio") || !strings.Contains(string(output), "\n1,") {
			t.Fatalf("%s delivered no decoded audio", phase)
		}
		t.Logf("%s: all %d decoded video frame hashes match; audio also decodes", phase, len(original))
	}
}

func hevcTools(t *testing.T) (string, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the real HEVC HTTP journey")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the real HEVC HTTP journey")
	}
	return ffmpeg, ffprobe
}

func hevcPlaybackSource(t *testing.T, request func(string, string, any, int) []byte) (string, string) {
	t.Helper()
	var listing struct{ Items []struct{ ID string } }
	if err := json.Unmarshal(request(http.MethodGet, "/api/v1/library", nil, http.StatusOK), &listing); err != nil || len(listing.Items) != 1 {
		t.Fatalf("populated library: %v, %d items", err, len(listing.Items))
	}
	item := "/api/v1/items/" + listing.Items[0].ID
	var plan struct {
		Compatible     string
		CompatiblePlan struct{ Mode string }
	}
	if err := json.Unmarshal(request(http.MethodGet, item+"/playback?videoCodecs=hevc,h264&audioCodecs=aac&audioChannels=2", nil, http.StatusOK), &plan); err != nil || plan.CompatiblePlan.Mode != "audio-transcode" || !strings.HasPrefix(plan.Compatible, "/hls/") {
		t.Fatalf("HEVC audio conversion plan: %v, mode %q", err, plan.CompatiblePlan.Mode)
	}
	return item, plan.Compatible
}

func hevcCommand(t *testing.T, ctx context.Context, commands *[][]string, artifacts map[string][]byte, name, executable string, arguments ...string) []byte {
	t.Helper()
	*commands = append(*commands, append([]string{executable}, arguments...))
	command := exec.CommandContext(ctx, executable, arguments...)
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	output, err := command.Output()
	artifacts[name+".stdout"] = output
	artifacts[name+".stderr"] = diagnostic.Bytes()
	if err != nil {
		t.Fatalf("%s: %v: %s", name, err, diagnostic.String())
	}
	return output
}

func hevcRequest(t *testing.T, ctx context.Context, host *httptest.Server, method, path string, body any, status int) []byte {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, method, host.URL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := host.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode != status {
		t.Fatalf("%s route: status %d, read %v: %s", method, response.StatusCode, err, payload)
	}
	return payload
}

func awaitHEVCPreparation(t *testing.T, request func(string, string, any, int) []byte, item, compatible string, artifacts map[string][]byte) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		payload := request(http.MethodPost, item+"/playback-prepare", map[string]string{"source": compatible}, http.StatusAccepted)
		artifacts["preparation.json"] = payload
		var state struct{ State string }
		if err := json.Unmarshal(payload, &state); err != nil {
			t.Fatal(err)
		}
		if state.State == "ready" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("short HEVC preparation never became ready: %q", state.State)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func hevcVideoHashes(output []byte) []string {
	var hashes []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "0,") {
			fields := strings.Split(line, ",")
			hashes = append(hashes, strings.TrimSpace(fields[len(fields)-1]))
		}
	}
	return hashes
}

func retainHEVCJourney(t *testing.T, commands [][]string, artifacts map[string][]byte, source string) {
	t.Helper()
	directory := os.Getenv("KINOSAIL_QA_HEVC_EVIDENCE")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o700); err != nil { //nolint:gosec // The test owner explicitly selects the private evidence directory.
		t.Error(err)
		return
	}
	if content, err := os.ReadFile(source); err == nil {
		artifacts["synthetic-source.mkv"] = content
	}
	if content, err := os.ReadFile("startup_hevc_e2e_test.go"); err == nil {
		artifacts["test-source.go"] = content
	} else {
		t.Error(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	revision, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Error(err)
	}
	hashes := map[string]string{}
	for name, content := range artifacts {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil { //nolint:gosec // The directory is test-owner input; artifact names are fixed by this journey.
			t.Error(err)
		}
		sum := sha256.Sum256(content)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	receipt, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "revision": strings.TrimSpace(string(revision)), "commands": commands, "repeat": "go test -race -count=1 -run '^TestShortHEVCStartupPreparationPreservesEveryDecodedVideoFrame$' ./internal/server", "environment": []string{runtime.GOOS, runtime.GOARCH, runtime.Version()}, "failed": t.Failed(), "sha256": hashes, "method": "real codecs and production loopback HTTP; no browser or physical-device proof"}, "", "  ")
	if err := os.WriteFile(filepath.Join(directory, "receipt.json"), receipt, 0o600); err != nil { //nolint:gosec // The test owner explicitly selects the private evidence directory.
		t.Error(err)
	}
}
