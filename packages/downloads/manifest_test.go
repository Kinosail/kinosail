package downloads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestManifestSealsExactBlocksAndFinalPartialBlock(t *testing.T) { //nolint:cyclop // Full and partial chunk assertions describe one sealed media file.
	t.Parallel()
	content := append(bytes.Repeat([]byte{17}, int(ChunkSize)), []byte("last block")...)
	path := filepath.Join(t.TempDir(), "media.mp4")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := sealManifestContext(t.Context(), path, "0123456789abcdef")
	whole := sha256.Sum256(content)
	last := sha256.Sum256([]byte("last block"))
	if err != nil || manifest.Size != int64(len(content)) || len(manifest.Chunks) != 2 || manifest.SHA256 != hex.EncodeToString(whole[:]) || manifest.Chunks[1] != hex.EncodeToString(last[:]) {
		t.Fatalf("manifest=%#v err=%v", manifest, err)
	}
	job := Job{ID: manifest.ID, File: path, Size: manifest.Size, SHA256: manifest.SHA256, Title: "Movie"}
	if err := persistJSON(path+".manifest", manifest); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/file", nil)
	request.Header.Set("Range", "bytes=8388608-8388617")
	result := httptest.NewRecorder()
	if err := Serve(result, request, job); err != nil || result.Code != 206 || result.Body.String() != "last block" || result.Header().Get("Content-Digest") == "" {
		t.Fatalf("range=%d %q err=%v", result.Code, result.Body.String(), err)
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/file", nil)
	result = httptest.NewRecorder()
	if err := Serve(result, request, job); err != nil || result.Body.Len() != 0 {
		t.Fatalf("HEAD body=%d err=%v", result.Body.Len(), err)
	}
}

func TestManifestRejectsInvalidMetadataAndEmptyAssets(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "empty.mp4")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sealManifestContext(t.Context(), path, "0123456789abcdef"); err == nil {
		t.Fatal("empty file accepted")
	}
	good := Manifest{Version: 1, ID: "0123456789abcdef", Size: 1, SHA256: hex.EncodeToString(make([]byte, 32)), ChunkSize: ChunkSize, Chunks: []string{hex.EncodeToString(make([]byte, 32))}}
	job := Job{ID: good.ID, Size: 1, SHA256: good.SHA256, File: path}
	cases := []func(*Manifest){func(m *Manifest) { m.Version = 2 }, func(m *Manifest) { m.ID = "../bad" }, func(m *Manifest) { m.Size = 0 }, func(m *Manifest) { m.Size = ChunkSize*MaximumChunks + 1 }, func(m *Manifest) { m.ChunkSize = 1 }, func(m *Manifest) { m.Chunks = []string{"bad"} }, func(m *Manifest) { m.Chunks = nil }, func(m *Manifest) { m.SHA256 = "bad" }}
	for _, mutate := range cases {
		candidate := good
		mutate(&candidate)
		if validManifest(candidate, job) {
			t.Fatal("invalid manifest accepted")
		}
	}
	data, _ := json.Marshal(good)
	data = append(data[:len(data)-1], []byte(`,"unknown":true}`)...)
	if err := os.WriteFile(path+".manifest", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(job); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
}

func TestManifestAuthorizationAndSourceReplacement(t *testing.T) { //nolint:cyclop // Authorization and replacement checks share one admitted download.
	t.Parallel()
	source := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(source, []byte("original bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON})
	item := library.Item{ID: "film", Kind: "video", Title: "Film", Path: source, Added: time.Now()}
	first, err := manager.Start("viewer", item, "original")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ready, err := manager.Wait(ctx, "viewer", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Manifest("other", ready.ID); err == nil {
		t.Fatal("another viewer accessed manifest")
	}
	if _, err := manager.Manifest("viewer", "../file"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if err := os.WriteFile(source, []byte("a replacement with different size"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := manager.Start("viewer", item, "original")
	if err != nil || second.ID == first.ID {
		t.Fatalf("revision reused: %v", err)
	}
	manifest, err := manager.Manifest("viewer", first.ID)
	if err != nil || manifest.SHA256 != ready.SHA256 {
		t.Fatalf("old revision changed: %v", err)
	}
	old, err := os.ReadFile(ready.File)
	if err != nil || string(old) != "original bytes" {
		t.Fatalf("old bytes=%q err=%v", old, err)
	}
	if err := manager.Remove("viewer", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Manifest("viewer", first.ID); err == nil {
		t.Fatal("removed manifest returned")
	}
	if _, err := os.Stat(ready.File + ".manifest"); !os.IsNotExist(err) {
		t.Fatal("removed manifest leaked")
	}
}

func TestTrackSelectionRejectsBeforeQueueAndPreservesSubtitles(t *testing.T) { //nolint:cyclop // Track rejections must leave the same queue and output fixture untouched.
	t.Parallel()
	facts := playback.MediaFacts{Video: playback.VideoFacts{Codec: "h264"}, Audio: []playback.AudioFacts{{Index: 0, SourceIndex: 1}, {Index: 1, SourceIndex: 2}}, Subtitles: []playback.SubtitleFacts{{Index: 0, SourceIndex: 3, Codec: "hdmv_pgs_subtitle"}, {Index: 1, SourceIndex: 4, Codec: "subrip", Text: true}}}
	item := library.Item{ID: "film", Kind: "video", Title: "Film", Path: "/media/film.mkv", Added: time.Now()}
	cache := t.TempDir()
	manager := New(Config{Context: t.Context(), Cache: cache, Persist: persistJSON, Inspect: func(context.Context, library.Item) playback.MediaFacts { return facts }})
	cases := []*TrackSelection{{Audio: []int{-1}, Subtitles: []int{}}, {Audio: []int{2}, Subtitles: []int{}}, {Audio: []int{1, 1}, Subtitles: []int{}}, {Audio: []int{}, Subtitles: []int{99}}, {Audio: make([]int, 33), Subtitles: []int{}}, {Audio: nil, Subtitles: []int{}}}
	for _, selection := range cases {
		if _, err := manager.StartSelected("viewer", item, "720p", selection); err == nil {
			t.Fatal("invalid selection queued")
		}
	}
	if len(manager.List("viewer")) != 0 {
		t.Fatal("invalid selection mutated jobs")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid selection wrote files")
	}
	_, maps, mkv, err := resolveTracks(facts, item, &TrackSelection{Audio: []int{1}, Subtitles: []int{0}})
	if err != nil || !mkv || len(maps) != 6 || maps[3] != "0:2" || maps[5] != "0:3" {
		t.Fatalf("track mapping=%v mkv=%v err=%v", maps, mkv, err)
	}
	_, maps, mkv, err = resolveTracks(facts, item, &TrackSelection{Audio: []int{}, Subtitles: []int{1}})
	if err != nil || mkv || len(maps) != 4 || maps[3] != "0:4" {
		t.Fatalf("text mapping=%v mkv=%v err=%v", maps, mkv, err)
	}
	selection := TrackSelection{Audio: []int{}, Subtitles: []int{}}
	raw, _ := json.Marshal(selection)
	var restored TrackSelection
	if err := json.Unmarshal(raw, &restored); err != nil || !validateSelection(&restored) || len(restored.Audio) != 0 || restored.Audio == nil {
		t.Fatal("empty selection became all tracks")
	}
}

func TestServeConditionalRangeUsesResponseIntegrity(t *testing.T) {
	t.Parallel()
	content := append(bytes.Repeat([]byte{17}, int(ChunkSize)), []byte("last block")...)
	path := filepath.Join(t.TempDir(), "media.mp4")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := sealManifestContext(t.Context(), path, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistJSON(path+".manifest", manifest); err != nil {
		t.Fatal(err)
	}
	job := Job{ID: manifest.ID, File: path, Size: manifest.Size, SHA256: manifest.SHA256, Title: "Movie"}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	etag := `"` + job.SHA256 + `"`
	for _, test := range []struct {
		name, validator, condition, rangeValue string
		code                                   int
		digest                                 bool
	}{
		{"matching tag", etag, "", "bytes=8388608-8388617", 206, true},
		{"replaced tag", `"old"`, "", "bytes=8388608-8388617", 200, false},
		{"weak tag", "W/" + etag, "", "bytes=8388608-8388617", 200, false},
		{"invalid validator", "invalid", "", "bytes=8388608-8388617", 200, false},
		{"matching date", info.ModTime().UTC().Format(http.TimeFormat), "", "bytes=8388608-8388617", 206, true},
		{"old date", info.ModTime().Add(-time.Hour).UTC().Format(http.TimeFormat), "", "bytes=8388608-8388617", 200, false},
		{"completed replaced", `"old"`, "", "bytes=" + strconv.FormatInt(job.Size, 10) + "-", 200, false},
		{"completed matching", etag, "", "bytes=" + strconv.FormatInt(job.Size, 10) + "-", 416, false},
		{"failed precondition", etag, `"old"`, "bytes=8388608-8388617", 412, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/file", nil)
			request.Header.Set("Range", test.rangeValue)
			request.Header.Set("If-Range", test.validator)
			request.Header.Set("If-Match", test.condition)
			result := httptest.NewRecorder()
			if err := Serve(result, request, job); err != nil {
				t.Fatal(err)
			}
			assertConditionalStatus(t, result, test.code, test.digest)
			assertConditionalBody(t, result, content)
		})
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, content) {
		t.Fatal("conditional request changed sealed media")
	}
}

func assertConditionalBody(t *testing.T, result *httptest.ResponseRecorder, content []byte) {
	t.Helper()
	if result.Code == http.StatusOK && !bytes.Equal(result.Body.Bytes(), content) {
		t.Fatal("replacement did not return complete representation")
	}
	if result.Code == http.StatusPartialContent && result.Body.String() != "last block" {
		t.Fatal("matching validator returned wrong block")
	}
}

func assertConditionalStatus(t *testing.T, result *httptest.ResponseRecorder, code int, digest bool) {
	t.Helper()
	if result.Code != code || (result.Header().Get("Content-Digest") != "") != digest {
		t.Fatalf("status=%d digest=%q; want %d digest=%v", result.Code, result.Header().Get("Content-Digest"), code, digest)
	}
}
