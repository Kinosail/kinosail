package markers

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMarkerStoreSuppressionAndManualReplacement(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "item", Size: 1, Added: time.Unix(1, 0)}
	intro := Marker{Type: "intro", Label: "Intro", Start: 1, End: 2, Source: "manual"}
	credits := Marker{Type: "credits", Label: "Credits", Start: 90, End: 100, Source: "manual"}
	analyzer := NewAnalyzer(Config{})
	analyzer.records[item.ID] = Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion, Markers: []Marker{intro, credits}, Suppressed: []string{"intro"}}
	chapters := []Marker{{Type: "intro", Label: "Intro", Start: 3, End: 4, Source: "chapter"}, credits}
	if got := analyzer.Markers(item, chapters); !reflect.DeepEqual(got, []Marker{credits}) {
		t.Fatalf("suppressed markers = %#v", got)
	}
	if err := analyzer.SetManual(item, "intro", 5, 10, 100); err != nil {
		t.Fatal(err)
	}
	markers := analyzer.Markers(item, nil)
	if len(markers) != 2 || markers[0] != (Marker{Type: "intro", Label: "Intro", Start: 5, End: 10, Source: "manual"}) || markers[1] != credits {
		t.Fatalf("manual replacement = %#v", markers)
	}
	if len(analyzer.records[item.ID].Suppressed) != 0 {
		t.Fatalf("manual replacement remained suppressed: %#v", analyzer.records[item.ID].Suppressed)
	}
	if err := analyzer.Suppress(item, "intro"); err != nil {
		t.Fatal(err)
	}
	if got := analyzer.Markers(item, nil); !reflect.DeepEqual(got, []Marker{credits}) {
		t.Fatalf("markers after suppression = %#v", got)
	}
}
