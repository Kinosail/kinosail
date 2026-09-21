package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestPlannedHLSLoadingDoesNotRepeatPlaybackEnrichment(t *testing.T) {
	for _, operation := range []string{"playlist", "cached segment", "encode", "software recovery"} {
		t.Run(operation, func(t *testing.T) {
			manager, item, chapterCalls, arguments := hlsLoadingFixture(t)
			recipe := hlsRecipe{mode: "remux"}
			options, err := manager.settings.transcodingFor("")
			if err != nil {
				t.Fatal(err)
			}
			key := hlsRecipeKey(item.ID, recipe)
			directory := filepath.Join(manager.cache, key)
			switch operation {
			case "playlist":
				identity := options.Cache + ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=11"
				writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+identity+"\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\n1080p/index.m3u8\n")
				writeHLSLoadingFile(t, filepath.Join(directory, ".seekable"), identity)
				err = manager.prepare(t.Context(), item, recipe)
			case "cached segment":
				writeHLSLoadingFile(t, filepath.Join(directory, "1080p/segment-00075.m4s"), "segment")
				err = manager.prepareSegment(t.Context(), item, recipe, "1080p/segment-00075.m4s")
			case "encode":
				err = manager.encodeVariants(t.Context(), item, directory, options, recipe, 0)
			case "software recovery":
				options.Accelerator = "cuda"
				options.HardwareDecode = true
				recipe.mode = "transcode"
				job := &hlsJob{err: errors.New("device setup failed")}
				if !manager.retrySoftwareHLSEncode(t.Context(), item, job, directory, options, recipe, 0, false) {
					t.Fatalf("software recovery failed: %v", job.err)
				}
				err = job.err
			}
			if err != nil {
				t.Fatal(err)
			}
			assertNoHLSLoadingEnrichment(t, chapterCalls, arguments)
		})
	}
}

func TestPlannedHLSLoadingStillRejectsInvalidSourcesBeforeOutput(t *testing.T) {
	for _, operation := range []string{"playlist", "segment"} {
		for _, invalid := range []hlsRecipe{{mode: "remux", audio: 1}, {mode: "remux", offset: 120}, {mode: "transcode", burn: "text", subtitle: 3}} {
			t.Run(operation+"/"+invalid.token(), func(t *testing.T) {
				assertInvalidHLSLoading(t, operation, invalid)
			})
		}
	}
}

func TestHLSFactsDoNotRemovePlaybackPlanningEnrichment(t *testing.T) {
	manager, item, calls, arguments := hlsLoadingFixture(t)
	_ = manager.probe.inspect(t.Context(), item)
	used, err := os.ReadFile(arguments)
	if err != nil || calls.Load() != 1 || !strings.Contains(string(used), "-show_frames") {
		t.Fatalf("planning enrichment = %d calls, arguments %q, error %v", calls.Load(), used, err)
	}
}

func hlsLoadingFixture(t *testing.T) (*hlsManager, library.Item, *atomic.Int32, string) {
	t.Helper()
	var chapterCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		chapterCalls.Add(1)
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	tools := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Path: filepath.Join(tools, "Episode.mkv"), Kind: "video", Show: "Example", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
	writeHLSLoadingFile(t, item.Path, "media")
	arguments := filepath.Join(tools, "probe-arguments")
	executable := filepath.Join(tools, "ffprobe")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + arguments + "'\n" + `case " $* " in
  *" -show_frames "*) printf '%s' '{"frames":[]}' ;;
  *) printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac"}],"chapters":[{"start_time":"0","end_time":"10","tags":{"title":"Intro"}},{"start_time":"10","end_time":"120","tags":{"title":"Chapter 2"}}],"format":{"format_name":"matroska","duration":"120"}}' ;;
esac
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil { //nolint:gosec // Executable local probe fixture.
		t.Fatal(err)
	}
	probe := newMediaProbe(executable)
	probe.chapters = newChapterProvider(provider.URL + "/api/v1")
	if facts := probe.core.Facts(t.Context(), item); facts.Duration != 120 {
		t.Fatalf("fixture source facts = %#v", facts)
	}
	ffmpeg := filepath.Join(tools, "ffmpeg")
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\n"+servertest.PlayableHLS()), 0o700); err != nil { //nolint:gosec // Executable local encoder fixture.
		t.Fatal(err)
	}
	settings := &settingsStore{}
	settings.value.Selection = transcodehardware.Selection{Accelerator: "none"}
	manager := newHLS(t.Context(), t.TempDir(), ffmpeg, nil, probe, settings, workload.New(1))
	return manager, item, &chapterCalls, arguments
}

func assertNoHLSLoadingEnrichment(t *testing.T, calls *atomic.Int32, arguments string) {
	t.Helper()
	used, err := os.ReadFile(arguments)
	if err != nil || calls.Load() != 0 || len(strings.Split(strings.TrimSpace(string(used)), "\n")) != 1 || strings.Contains(string(used), "-show_frames") {
		t.Fatalf("HLS repeated planning work: chapter calls=%d, probe arguments=%q, error=%v", calls.Load(), used, err)
	}
}

func writeHLSLoadingFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertInvalidHLSLoading(t *testing.T, operation string, invalid hlsRecipe) {
	t.Helper()
	manager, item, calls, arguments := hlsLoadingFixture(t)
	var err error
	if operation == "playlist" {
		err = manager.prepare(t.Context(), item, invalid)
	} else {
		err = manager.prepareSegment(t.Context(), item, invalid, "1080p/segment-00001.m4s")
	}
	if !errors.Is(err, playback.ErrHLSSource) {
		t.Fatalf("invalid source error = %v", err)
	}
	entries, readErr := os.ReadDir(manager.cache)
	if readErr != nil || len(entries) != 0 || len(manager.jobs) != 0 {
		t.Fatalf("invalid source produced output: entries=%v jobs=%d error=%v", entries, len(manager.jobs), readErr)
	}
	assertNoHLSLoadingEnrichment(t, calls, arguments)
}

func TestHardwareHLSRecoveryPreservesPublishedOutput(t *testing.T) {
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "manifest", true: "published segments"}[preserve], func(t *testing.T) { assertPublishedHLSRetained(t, preserve) })
	}
}

func assertPublishedHLSRetained(t *testing.T, preserve bool) {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixture(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "index.m3u8")
	if !preserve {
		writeHLSLoadingFile(t, path, "published")
	}
	options, err := manager.settings.transcodingFor("")
	if err != nil {
		t.Fatal(err)
	}
	options.Accelerator = "cuda"
	options.HardwareDecode = true
	original := errors.New("device setup failed")
	job := &hlsJob{err: original}
	if manager.retrySoftwareHLSEncode(t.Context(), item, job, directory, options, hlsRecipe{mode: "transcode"}, 0, preserve) || !errors.Is(job.err, original) {
		t.Fatal("published job was retried")
	}
	if !preserve {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "published" {
			t.Fatalf("published manifest changed: %q %v", data, err)
		}
	}
}
