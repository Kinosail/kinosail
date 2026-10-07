package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Hosted codecs and real public routes under trusted configuration protect refill.
func TestCopiedRecoveryRealFirstFragmentRegenerationReopens(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("Real media/build proof runs on the hosted runner; focused local tests use controlled stand-ins")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("hosted FFmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("hosted FFprobe unavailable")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=24:d=12", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=12", "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0", "-c:a", "aac", "-avoid_negative_ts", "disabled", filepath.Join(media, "Episode.S01E01.mp4")) //nolint:gosec // Fixed bounded synthetic input and discovered hosted codec.
	if err := command.Run(); err != nil {
		t.Fatal("bounded source fixture failed")
	}
	source := filepath.Join(media, "Episode.S01E01.mp4")
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	owned := filepath.Join(tools, "owned-pids")
	adapter := filepath.Join(tools, "ffmpeg")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	body := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + quote(owned) + "\nexec " + quote(ffmpeg) + " \"$@\"\n"
	if err := os.WriteFile(adapter, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	} //nolint:gosec // Owned hosted adapter records process identity and execs the real codec unchanged.
	workers, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	cache := t.TempDir()
	config := server.Config{Lifecycle: workers, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe}
	handler, id := formatTestItem(t, config)
	var info struct{ Compatible string }
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
	for {
		var preparation struct{ State string }
		mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/items/"+id+"/playback-prepare", map[string]any{"source": info.Compatible}), &preparation)
		if preparation.State == "ready" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("prepared copied cache not ready")
		case <-time.After(20 * time.Millisecond):
		}
	}
	master := speedTestGET(t, ctx, handler, info.Compatible)
	variants := speedTestURIs(master)
	if len(variants) != 1 {
		t.Fatal("fixture needs one copied rendition")
	}
	variantURL := info.Compatible[:len(info.Compatible)-len("index.m3u8")] + variants[0]
	base := variantURL[:len(variantURL)-len("index.m3u8")]
	beforePlaylist := speedTestGET(t, ctx, handler, variantURL)
	for _, name := range speedTestURIs(beforePlaylist) {
		speedTestGET(t, ctx, handler, base+name)
	}
	beforeOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	beforeInit := speedTestGET(t, ctx, handler, base+"init.mp4")
	beforeFirst := speedTestGET(t, ctx, handler, base+"segment-00000.m4s")
	roots, err := filepath.Glob(filepath.Join(cache, id+"-plan-*"))
	if err != nil || len(roots) != 1 {
		t.Fatal("exact copied recipe cache missing")
	}
	if _, err := os.Stat(filepath.Join(roots[0], ".copy-timeline")); err != nil {
		t.Fatal("public fixture did not select indexed copied preparation")
	}
	firstPath := filepath.Join(roots[0], filepath.Dir(variants[0]), "segment-00000.m4s")
	remaining := filepath.Join(filepath.Dir(firstPath), "segment-00001.m4s")
	retained, err := os.Stat(remaining)
	if err != nil {
		t.Fatal(err)
	}
	retainedBytes, err := os.ReadFile(remaining)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(firstPath); err != nil {
		t.Fatal(err)
	}
	regenerated := speedTestGET(t, ctx, handler, base+"segment-00000.m4s")
	afterOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	if afterOwned <= beforeOwned {
		t.Fatal("deleted segment0 did not execute a real regeneration worker")
	}
	cancelWorkers()
	config.Lifecycle = ctx
	reopened, reopenedID := formatTestItem(t, config)
	if reopenedID != id {
		t.Fatal("cold reopen changed source identity")
	}
	speedTestGET(t, ctx, reopened, info.Compatible)
	afterPlaylist := speedTestGET(t, ctx, reopened, variantURL)
	afterInit := speedTestGET(t, ctx, reopened, base+"init.mp4")
	if copiedRecoveryJoinedCodecs(t, ctx, owned) != afterOwned {
		t.Fatal("cold reopen restarted an accepted indexed cache")
	}
	afterRemaining, err := os.Stat(remaining)
	if err != nil || !os.SameFile(retained, afterRemaining) {
		t.Fatal("refill/reopen discarded an existing indexed fragment")
	}
	afterBytes, err := os.ReadFile(remaining)
	if err != nil || !bytes.Equal(retainedBytes, afterBytes) {
		t.Fatal("refill changed an existing fragment's bytes")
	}
	afterSource, err := os.Stat(source)
	afterSourceBytes, readErr := os.ReadFile(source)
	if err != nil || readErr != nil || !os.SameFile(sourceInfo, afterSource) || sourceInfo.Size() != afterSource.Size() || !sourceInfo.ModTime().Equal(afterSource.ModTime()) || !bytes.Equal(sourceBytes, afterSourceBytes) {
		t.Fatal("public refill/reopen mutated its source snapshot")
	}
	if !bytes.Equal(beforePlaylist, afterPlaylist) || !bytes.Equal(beforeInit, afterInit) || !bytes.Equal(beforeFirst, regenerated) {
		t.Fatal("segment0 refill changed committed metadata/init/media")
	}
	t.Logf("real copied segment0 delete/regenerate/cold-reopen: playlist=%x init=%x first=%x source=%x existing-fragment-retained=true owned-codecs-joined=%d", sha256.Sum256(afterPlaylist), sha256.Sum256(afterInit), sha256.Sum256(regenerated), sha256.Sum256(afterSourceBytes), afterOwned)
}

func copiedRecoveryJoinedCodecs(t *testing.T, ctx context.Context, path string) int {
	t.Helper()
	stable := 0
	for {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal("owned codec identity receipt missing")
		}
		identities := strings.Fields(string(data))
		joined := len(identities) > 0
		for _, identity := range identities {
			pid, err := strconv.Atoi(identity)
			if err != nil {
				t.Fatal("invalid owned codec identity")
			}
			process, err := os.FindProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			if err := process.Signal(syscall.Signal(0)); !os.IsNotExist(err) && err != syscall.ESRCH {
				joined = false
			}
		}
		if joined {
			stable++
		} else {
			stable = 0
		}
		if stable == 3 {
			return len(identities)
		}
		select {
		case <-ctx.Done():
			t.Fatal("owned real codec did not join")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
