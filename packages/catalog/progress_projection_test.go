package catalog

import (
	"strconv"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProgressProjectionKeepsViewerIsolationAndHistoryOrder(t *testing.T) {
	now := time.Unix(100, 0)
	values := map[string]PlaybackState{"legacy": {Seconds: 3600, Updated: now}, "viewer:new": {Seconds: 60, Updated: now.Add(time.Minute)}, "viewer:watched": {Seconds: 2, Watched: true}, "viewer:dismissed": {Seconds: 3, Dismissed: true}}
	items := []library.Item{{ID: "legacy"}, {ID: "new"}, {ID: "watched"}, {ID: "dismissed"}, {ID: "empty"}}
	if ViewerProgress(values, "other", false, "legacy").Seconds != 0 || ViewerProgress(values, "viewer", true, "legacy").Seconds != 3600 {
		t.Fatal("legacy fallback leaked")
	}
	active := ActiveProgress(values, "viewer", true, items)
	if len(active) != 2 || active[0].Resume != "Resume at 1h 0m" || active[1].Resume != "Resume at 1m" {
		t.Fatalf("active = %#v", active)
	}
	history := ProgressHistory(values, "viewer", true, items)
	if len(history) != 2 || history[0].ID != "new" || history[1].ID != "legacy" {
		t.Fatalf("history = %#v", history)
	}
	if len(ProgressHistory(values, "viewer", true, history)) != 2 {
		t.Fatal("complete history truncated")
	}
}

func TestRecentAdminProgressFiltersSortsAndBounds(t *testing.T) {
	values := map[string]PlaybackState{"legacy": {}, "owner:missing": {Updated: time.Now()}, "owner:empty": {}}
	items := []library.Item{{ID: "empty", Title: "Empty"}}
	for i := range 25 {
		id := strconv.Itoa(i)
		items = append(items, library.Item{ID: id, Title: "Title " + id})
		values["owner:"+id] = PlaybackState{Seconds: 60, Updated: time.Unix(int64(i+1), 0)}
	}
	result := RecentAdminProgress(values, items, map[string]string{"owner": "Owner"})
	if len(result) != 20 || result[0].Title != "Title 24" || result[19].Title != "Title 5" || result[0].Profile != "Owner" || result[0].Position != "Resume at 1m" || result[0].Updated == "" {
		t.Fatalf("admin history = %#v", result)
	}
}

func TestResumeAtPreservesSubminutePositions(t *testing.T) {
	for seconds, want := range map[float64]string{0: "Resume at 0:00", 2.9: "Resume at 0:02", 59: "Resume at 0:59", 60: "Resume at 1m", 3660: "Resume at 1h 1m"} {
		if got := ResumeAt(seconds); got != want {
			t.Errorf("ResumeAt(%v) = %q, want %q", seconds, got, want)
		}
	}
}
