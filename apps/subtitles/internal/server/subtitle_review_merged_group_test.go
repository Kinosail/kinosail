package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUIStateFixturesSubtitleMergedGroupRetainsEverySourceCue(t *testing.T) {
	t.Parallel()
	const count = 10000
	var input strings.Builder
	for index := range count {
		_, _ = fmt.Fprintf(&input, "%d\n%s --> %s\nRepeated dialogue\n\n", index+1, subtitleMergedFixtureTime(index), subtitleMergedFixtureTime(index+1))
	}
	initial := input.String()
	handler, base, target := subtitleInspectorFixture(t, initial)
	response := requestJSON(t, handler, http.MethodPost, base+"/preview", `{"language":"en","mergeRepeated":true}`)
	var review subtitlePairingAPIReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &review) != nil {
		t.Fatalf("long merged preview = %d %s", response.Code, response.Body.String())
	}
	assertSubtitleMergedSources(t, review, count)
	dir := os.Getenv("KINOSAIL_SUBTITLE_MERGED_FIXTURE_DIR")
	if root := os.Getenv("KINOSAIL_UI_FIXTURE_DIR"); dir == "" && root != "" {
		dir = filepath.Join(root, "subtitle-merged")
	}
	writeSubtitlePairingBrowserFixtureTo(t, handler, base, dir, response.Body.Bytes(), nil)
	current, err := os.ReadFile(target)
	if err != nil || string(current) != initial {
		t.Fatalf("long preview changed its installed data: %v", err)
	}
	if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
		t.Fatal("long preview created a recovery sidecar")
	}
}

func assertSubtitleMergedSources(t *testing.T, review subtitlePairingAPIReview, count int) {
	t.Helper()
	if len(review.Current.Cues) != count || len(review.Proposed.Cues) != 1 || len(review.Comparison) != 1 {
		t.Fatalf("long source/proposed/comparison counts = %d/%d/%d", len(review.Current.Cues), len(review.Proposed.Cues), len(review.Comparison))
	}
	group := review.Comparison[0]
	if group.Kind != "merged" || len(group.Current) != count || len(group.Proposed) != 1 || group.Proposed[0] != 0 {
		t.Fatalf("long merged source mapping has incomplete identities: kind=%q current=%d proposed=%v", group.Kind, len(group.Current), group.Proposed)
	}
	for index, source := range group.Current {
		cue := review.Current.Cues[index]
		if source != index || cue.Start != float64(index) || cue.End != float64(index+1) || cue.Text != "Repeated dialogue" {
			t.Fatalf("source cue %d lost its identity/time/text: mapping=%d cue=%+v", index, source, cue)
		}
	}
	proposed := review.Proposed.Cues[0]
	if proposed.Start != 0 || proposed.End != float64(count) || proposed.Text != "Repeated dialogue" {
		t.Fatalf("long proposed dialogue = %+v", proposed)
	}
}

func subtitleMergedFixtureTime(seconds int) string {
	return fmt.Sprintf("%02d:%02d:%02d,000", seconds/3600, seconds/60%60, seconds%60)
}
