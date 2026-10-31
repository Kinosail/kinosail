package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func attentionFixture(t *testing.T) (*subtitleManager, []library.Item) {
	t.Helper()
	media, now := t.TempDir(), time.Now()
	settings := newSettingsStore("", "", nil)
	settings.value.SubtitleLanguages = []string{"en"}
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), "", nil, settings, "")
	contents := []byte("1\n00:00:01,000 --> 00:00:02,000\nLine\n")
	var items []library.Item
	for _, fixture := range []struct {
		title, evidence string
		managed, listed bool
	}{
		{"Arrival", "unverified", true, true},
		{"Blade Runner", "audio", true, true},
		{"Contact", "unverified", false, true},
		{"Dune", "unverified", true, false},
	} {
		id := strings.Repeat(strings.ToLower(fixture.title[:1]), 16)
		video, sidecar := filepath.Join(media, fixture.title+".mp4"), filepath.Join(media, fixture.title+".en.srt")
		if err := os.WriteFile(sidecar, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(video, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
		item := library.Item{ID: id, Kind: "video", Title: fixture.title, Path: video}
		if fixture.listed {
			item.Subtitles = []string{sidecar}
		}
		items = append(items, item)
		provider.ledger.state.Records[subtitleRecordKey(id, "en")] = completeSubtitleRecord(sidecar, contents, subtitleRecord{Source: "subdl", Score: 80, CheckedAt: now.Unix(), InstalledAt: now.Unix(), Managed: fixture.managed, TimingEvidence: fixture.evidence, Synchronization: "none"})
	}
	index := memoryLibraryIndex(items, true)
	index.SetRoots([]libraryRoot{{Path: media}})
	provider.index = index
	manager := newSubtitleManager(index, settings, provider, nil)
	manager.operations = newSubtitleOperations(Config{DataDir: t.TempDir()})
	return manager, items
}

func dashboardProjection(t *testing.T, manager *subtitleManager, target string) subtitleDashboardData {
	t.Helper()
	request := ownerRequest(target)
	options, err := subtitleDashboardQuery(request)
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	data, err := manager.projection(request, options)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDashboardCountsAndFiltersSubtitlesWithUncheckedTiming(t *testing.T) {
	t.Parallel()
	manager, _ := attentionFixture(t)
	summary := dashboardProjection(t, manager, "/")
	if summary.TimingUnchecked != 1 {
		t.Fatalf("timing unchecked = %d, want 1 (managed, unverified, still installed)", summary.TimingUnchecked)
	}
	page := renderSummary(t, summary)
	if !strings.Contains(page, `href="/?view=library&amp;status=timing"`) || !strings.Contains(page, "Timing not checked") {
		t.Fatal("the summary does not link to subtitles with unchecked timing")
	}
	filtered := dashboardProjection(t, manager, "/?view=library&status=timing")
	if filtered.Matched != 1 || len(filtered.Items) != 1 || filtered.Items[0].Title != "Arrival" || !filtered.Items[0].TimingUnchecked {
		t.Fatalf("timing filter = %d %#v", filtered.Matched, filtered.Items)
	}
	if _, err := subtitleDashboardQuery(ownerRequest("/?view=wanted&status=timing")); err == nil {
		t.Fatal("the wanted view accepted the timing filter")
	}
}

func TestDashboardShowsTheWorkThatIsRunningNow(t *testing.T) {
	t.Parallel()
	manager, items := attentionFixture(t)
	if data := dashboardProjection(t, manager, "/"); len(data.Activity) != 0 || strings.Contains(renderSummary(t, data), "Working now") {
		t.Fatalf("idle activity = %#v", data.Activity)
	}
	manager.operations.records["running"] = subtitleOperationRecord{subtitleOperationReceipt: subtitleOperationReceipt{ID: "running", Action: "fetch", Item: items[0].ID, State: "running"}}
	manager.operations.records["prepared"] = subtitleOperationRecord{subtitleOperationReceipt: subtitleOperationReceipt{ID: "prepared", Action: "apply", Item: items[1].ID, State: "prepared"}}
	manager.operations.records["done"] = subtitleOperationRecord{subtitleOperationReceipt: subtitleOperationReceipt{ID: "done", Action: "maintain", State: "succeeded"}}
	manager.noteAutomationStart(time.Date(2026, 10, 30, 22, 0, 0, 0, time.UTC))
	data := dashboardProjection(t, manager, "/")
	if len(data.Activity) != 2 || data.Activity[0].Label != "Automatic check" || data.Activity[1].Label != "Finding a subtitle" || data.Activity[1].Title != "Arrival" {
		t.Fatalf("activity = %#v", data.Activity)
	}
	page := renderSummary(t, data)
	for _, fragment := range []string{"Working now", "Automatic check", "Finding a subtitle", "Arrival"} {
		if !strings.Contains(page, fragment) {
			t.Errorf("summary lacks %q", fragment)
		}
	}
	manager.noteAutomation(subtitleMaintenanceResult{}, time.Date(2026, 10, 30, 22, 1, 0, 0, time.UTC))
	if data := dashboardProjection(t, manager, "/"); len(data.Activity) != 1 || data.Automation.Running {
		t.Fatalf("activity after the automatic check = %#v", data.Activity)
	}
}
