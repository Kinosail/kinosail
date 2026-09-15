package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitlePreparationSelectsPreferredTextAndReusesCache(t *testing.T) {
	t.Parallel()
	for _, scenario := range []subtitlePreparationScenario{
		{"preferred", "fr", "video", "", `[{"index":2,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"fra"}}]`, "0:3"},
		{"fallback", "de", "video", "", `[{"index":2,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}}]`, "0:2"},
		{"sidecar", "fr", "video", "film.fr.srt", `[{"index":2,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}}]`, ""},
		{"image", "en", "video", "", `[{"index":2,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","tags":{"language":"eng"}}]`, ""},
		{"audio", "en", "audio", "", `[]`, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			checkSubtitlePreparation(t, scenario)
		})
	}
}

func TestSubtitlePreparationRejectsInvalidLanguageBeforeWork(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"", "not-a-language", "en_US", strings.Repeat("x", 1024)} {
		// A nil probe proves invalid preferences cannot reach probing or extraction.
		if err := (*mediaProbe)(nil).preparePreferredSubtitle(context.Background(), library.Item{Kind: "video"}, language); err == nil {
			t.Fatalf("accepted language %q", language)
		}
	}
}

func checkSubtitlePreparation(t *testing.T, scenario subtitlePreparationScenario) {
	t.Helper()
	root := t.TempDir()
	calls := filepath.Join(root, "calls")
	ffprobe, ffmpeg := filepath.Join(root, "ffprobe"), filepath.Join(root, "ffmpeg")
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":"+scenario.streams+",\"format\":{\"duration\":\"30\"}}'\n", 0o700)
	write(ffmpeg, "#!/bin/sh\nprintf '%s\\n' \"$@\" >> '"+calls+"'\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:02.000\\nCaptions\\n'\n", 0o700)
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv"), Kind: scenario.kind}
	write(item.Path, "media", 0o600)
	if scenario.sidecar != "" {
		item.Subtitles = []string{filepath.Join(root, scenario.sidecar)}
	}
	probe := newMediaProbe(ffprobe)
	probe.ffmpeg, probe.cacheDir = ffmpeg, filepath.Join(root, "cache")
	for range 2 {
		if err := probe.preparePreferredSubtitle(t.Context(), item, scenario.language); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(calls)
	if scenario.want == "" {
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected extraction: %q, %v", data, err)
		}
	} else if err != nil || strings.Count(string(data), scenario.want+"\n") != 1 {
		t.Fatalf("preferred extraction/cache calls = %q, %v", data, err)
	}
}

type subtitlePreparationScenario struct {
	name, language, kind, sidecar, streams, want string
}
