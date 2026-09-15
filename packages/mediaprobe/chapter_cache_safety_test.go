package mediaprobe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestCachedProbeReclassifiesStoryChaptersWithoutDecoding(t *testing.T) {
	t.Parallel()
	probe := New("missing-ffprobe")
	probe.ConfigureCache(t.TempDir())
	item := library.Item{ID: "movie", Kind: "video", Path: filepath.Join(t.TempDir(), "movie.mp4")}
	if err := os.WriteFile(item.Path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe.save(item, playback.SourceVersion(item.Path), Result{
		Duration: 200,
		Chapters: []Chapter{{Title: "Epilogue", Start: 100, End: 120}, {Title: "End Credits", Start: 120, End: 180}, {Title: "Post-credits scene", Start: 180, End: 200}},
		Markers:  []Marker{{Type: "outro", Label: "Outro", Start: 100, End: 120, Source: "chapter"}, {Type: "credits", Label: "Credits", Start: 120, End: 200, Source: "chapter"}},
	})
	got := probe.Facts(t.Context(), item)
	if len(got.Chapters) != 3 || len(got.Markers) != 1 || got.Markers[0].Start != 120 || got.Markers[0].End != 180 {
		t.Fatalf("cached classification = %#v", got)
	}
}
