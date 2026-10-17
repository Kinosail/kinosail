package markers

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSyntheticSeasonJourneyDetectsIntroOutroAndCreditRollWithRealTools(t *testing.T) {
	if os.Getenv("KINOSAIL_MARKER_MEDIA_E2E") == "" {
		t.Skip("set KINOSAIL_MARKER_MEDIA_E2E=1 to render a synthetic season with ffmpeg, fpcalc, and swift")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("the credit roll renderer uses AppKit")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is unavailable")
	}
	fpcalc, err := exec.LookPath("fpcalc")
	if err != nil {
		t.Skip("fpcalc is unavailable")
	}
	media := os.Getenv("KINOSAIL_MARKER_MEDIA_DIR")
	if media == "" {
		media = t.TempDir()
	}
	if _, err := os.Stat(filepath.Join(media, "checksums.txt")); err != nil { //nolint:gosec // The media directory is an operator-chosen local fixture folder.
		build := exec.CommandContext(t.Context(), "bash", filepath.Join("testdata", "synthetic-season.sh"), media) //nolint:gosec // The fixture script is checked in and the folder is operator-chosen.
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("synthetic season: %v\n%s", err, output)
		}
	}
	items := make([]library.Item, 0, 3)
	for episode := 1; episode <= 3; episode++ {
		path := filepath.Join(media, "episode-"+string(rune('0'+episode))+".mp4")
		info, err := os.Stat(path) //nolint:gosec // Rendered fixture inside the operator-chosen folder.
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, library.Item{ID: "episode-" + string(rune('0'+episode)), Kind: "video", Library: "Shows", Show: "Synthetic", Season: 1, Episode: episode, Path: path, Size: info.Size(), Added: time.Unix(int64(episode), 0)})
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir(), CacheDir: t.TempDir(), FFmpeg: ffmpeg, Fingerprint: fpcalc, Threads: 2})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 600} })
	started := time.Now()
	if err := analyzer.analyzeBatch(t.Context(), items, nil); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	results := make(map[string][]Marker, len(items))
	for _, item := range items {
		markers := analyzer.Markers(item, nil)
		results[item.ID] = markers
		found := map[string]Marker{}
		for _, marker := range markers {
			found[marker.Type] = marker
		}
		intro, outro, credits := found["intro"], found["outro"], found["credits"]
		if intro.Source != "fingerprint" || intro.Start > 2 || intro.End < 12 || intro.End > 18 {
			t.Errorf("%s intro = %#v", item.ID, intro)
		}
		if outro.Source != "fingerprint" || outro.Start < 518 || outro.Start > 523 || outro.End < 537 || outro.End > 542 {
			t.Errorf("%s outro = %#v", item.ID, outro)
		}
		if credits.Source != "text" || credits.Start < 538 || credits.Start > 546 || credits.End != 600 {
			t.Errorf("%s credits = %#v", item.ID, credits)
		}
	}
	checksums, _ := os.ReadFile(filepath.Join(media, "checksums.txt")) //nolint:gosec // Fixture checksum list inside the operator-chosen folder.
	artifact := map[string]any{"ffmpeg": ffmpeg, "fpcalc": fpcalc, "detectorVersion": DetectorVersion, "elapsed": elapsed.String(), "checksums": string(checksums), "markers": results}
	encoded, _ := json.MarshalIndent(artifact, "", "  ")
	t.Logf("SYNTHETIC_SEASON_ARTIFACT %s", encoded)
	if path := os.Getenv("KINOSAIL_MARKER_MEDIA_E2E_ARTIFACT"); path != "" {
		if err := os.WriteFile(path, encoded, 0o600); err != nil { //nolint:gosec // Artifact path is operator-chosen.
			t.Fatal(err)
		}
	}
}
