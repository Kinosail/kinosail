package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestAHindiSubtitleIsNotTreatedAsCaptions(t *testing.T) {
	t.Parallel()
	settings := newSettingsStore(t.TempDir(), "", nil)
	if err := settings.setSubtitlePlan("hi", "sdh"); err != nil {
		t.Fatal(err)
	}
	manager := &subtitleManager{settings: settings}
	item := library.Item{Kind: "video", Path: "/media/Lagaan.mp4", Subtitles: []string{"/media/Lagaan.hi.srt"}}
	if ready, _ := manager.planCoverage(t.Context(), item, "hi"); ready {
		t.Fatal("a standard Hindi subtitle satisfied the SDH preference")
	}
	item.Subtitles = append(item.Subtitles, "/media/Lagaan.hi.sdh.srt")
	if ready, _ := manager.planCoverage(t.Context(), item, "hi"); !ready {
		t.Fatal("Hindi SDH captions did not satisfy the SDH preference")
	}
	if role := subtitleRoleFromPath("/media/Lagaan.hi.srt"); role != "translation" {
		t.Fatalf("Lagaan.hi.srt role = %q", role)
	}
}
