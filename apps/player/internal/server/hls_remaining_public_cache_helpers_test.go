package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type remainingPublicAsset struct {
	info os.FileInfo
	data []byte
}

func remainingPublicSnapshot(t *testing.T, root string) map[string]remainingPublicAsset {
	t.Helper()
	assets := map[string]remainingPublicAsset{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		assets[path] = remainingPublicAsset{info: info, data: remainingPublicRead(t, path)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return assets
}

func remainingPublicSameSnapshot(t *testing.T, root string, before map[string]remainingPublicAsset) bool {
	t.Helper()
	after := remainingPublicSnapshot(t, root)
	if len(before) != len(after) {
		return false
	}
	for path, asset := range before {
		current, ok := after[path]
		if !ok || !os.SameFile(asset.info, current.info) || !asset.info.ModTime().Equal(current.info.ModTime()) ||
			asset.info.Size() != current.info.Size() || !bytes.Equal(asset.data, current.data) {
			return false
		}
	}
	return true
}

func remainingPublicAudio(t *testing.T, ctx context.Context, handler http.Handler, base string) []byte {
	t.Helper()
	variant := remainingPublicGet(t, ctx, handler, base+"index.m3u8", "", http.StatusOK)
	if !bytes.Contains(variant, []byte("#EXT-X-ENDLIST")) || !bytes.Contains(variant, []byte("#EXT-X-PLAYLIST-TYPE:VOD")) {
		t.Fatal("public AAC timeline was not completed VOD")
	}
	data := remainingPublicGet(t, ctx, handler, base+"init.mp4", "", http.StatusOK)
	names := speedTestURIs(variant)
	if len(names) < 5 || len(names) > 6 {
		t.Fatalf("public AAC fragment count = %d", len(names))
	}
	for _, name := range names {
		fragment := remainingPublicGet(t, ctx, handler, base+name, "", http.StatusOK)
		partial := remainingPublicGet(t, ctx, handler, base+name, "bytes=0-31", http.StatusPartialContent)
		if len(fragment) < 32 || !bytes.Equal(partial, fragment[:32]) {
			t.Fatal("public fragment Range did not return the same retained bytes")
		}
		data = append(data, fragment...)
	}
	return data
}

func remainingPublicGet(t *testing.T, ctx context.Context, handler http.Handler, route, byteRange string, status int) []byte {
	t.Helper()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, route, nil)
	if byteRange != "" {
		request.Header.Set("Range", byteRange)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("public cache asset returned%d, expected%d", response.Code, status)
	}
	return response.Body.Bytes()
}

func remainingPublicPCM(t *testing.T, ctx context.Context, ffmpeg string, data []byte) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-xerror", "-threads", "1", "-i", "pipe:0", "-map", "0:a:0", "-f", "s16le", "-") //nolint:gosec // Discovered codec and actual bounded generated public bytes.
	command.Stdin = bytes.NewReader(data)
	pcm, diagnostic := &remainingPublicBoundedOutput{maximum: 2 << 20}, &remainingPublicBoundedOutput{maximum: 65536}
	command.Stdout, command.Stderr = pcm, diagnostic
	if err := command.Run(); err != nil {
		t.Fatalf("full public AAC decode failed: %v", err)
	}
	return pcm.buffer.Bytes()
}

type remainingPublicBoundedOutput struct {
	buffer  bytes.Buffer
	maximum int
}

func (output *remainingPublicBoundedOutput) Write(data []byte) (int, error) {
	if len(data) > output.maximum-output.buffer.Len() {
		return 0, errors.New("public fixture output exceeds bound")
	}
	return output.buffer.Write(data)
}

func remainingPublicWaitEOF(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	for {
		data := remainingPublicRead(t, filepath.Join(root, "audio/index.m3u8"))
		if bytes.Contains(data, []byte("#EXT-X-ENDLIST")) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("physical AAC EOF was not published within the bounded cache proof")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func remainingPublicRead(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path) //nolint:gosec // Only owned disposable fixture paths.
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		t.Fatalf("bounded fixture read failed: %v", err)
	}
	return data
}

func remainingPublicWaitWorkers(t *testing.T, ctx context.Context, handler http.Handler, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	zeros := 0
	for zeros < 2 {
		allStopped := true
		for _, row := range strings.Split(strings.TrimSpace(string(remainingPublicRead(t, marker))), "\n") {
			pid, err := strconv.Atoi(strings.TrimPrefix(row, "call "))
			if err != nil || pid < 2 {
				t.Fatal("owned codec PID witness is invalid")
			}
			if !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
				allStopped = false
			}
		}
		metrics := remainingPublicGet(t, ctx, handler, "/settings/metrics", "", http.StatusOK)
		for _, class := range []string{"playback", "background"} {
			allStopped = allStopped && bytes.Contains(metrics, []byte("kinosail_workload_active{class=\""+class+"\"} 0\n"))
		}
		if allStopped {
			zeros++
		} else {
			zeros = 0
		}
		if time.Now().After(deadline) {
			t.Fatal("owned cache encoder did not join with two zero observations")
		}
		select {
		case <-ctx.Done():
			t.Fatal("owned cache worker completion exceeded proof deadline")
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func remainingPublicWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func remainingPublicHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func remainingPublicQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
