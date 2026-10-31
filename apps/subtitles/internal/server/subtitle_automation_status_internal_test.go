package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func renderSummary(t *testing.T, data subtitleDashboardData) string {
	t.Helper()
	data.View, data.Kind, data.Status = "summary", "all", "all"
	output := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if err := subtitleDashboardView.ExecuteTemplate(output, request, "content", data); err != nil {
		t.Fatal(err)
	}
	return output.Body.String()
}

func TestDashboardShowsTheLastAndNextAutomaticCheck(t *testing.T) {
	t.Parallel()
	manager := &subtitleManager{}
	if status := manager.automationStatus(); status.LastRun != "" || status.NextRun != "" {
		t.Fatalf("status before the first run = %#v", status)
	}
	page := renderSummary(t, subtitleDashboardData{Total: 3, Automation: manager.automationStatus()})
	if !strings.Contains(page, "The first automatic check runs after the library scan.") {
		t.Fatalf("summary before the first run lacks the schedule note")
	}
	ran := time.Date(2026, 10, 30, 22, 0, 0, 0, time.UTC)
	manager.noteAutomation(subtitleMaintenanceResult{Attempted: 4, Added: 2, Upgraded: 1, Failed: 1}, ran)
	status := manager.automationStatus()
	if status.LastRun != "2026-10-30T22:00:00Z" || status.NextRun != "2026-10-30T22:15:00Z" || status.Added != 2 || status.Upgraded != 1 || status.Failed != 1 {
		t.Fatalf("status after a run = %#v", status)
	}
	page = renderSummary(t, subtitleDashboardData{Total: 3, Automation: status})
	for _, fragment := range []string{`Last automatic check <time datetime="2026-10-30T22:00:00Z">`, "2 added, 1 upgraded, 1 not found", `Next check <time datetime="2026-10-30T22:15:00Z">`} {
		if !strings.Contains(page, fragment) {
			t.Errorf("summary lacks %q", fragment)
		}
	}
}

func TestProviderStatesUseReadableLabelsAndMatchingStateStyles(t *testing.T) {
	t.Parallel()
	page := renderSummary(t, subtitleDashboardData{Total: 1, Providers: []subtitleProviderHealth{
		{Name: "SubDL", State: "connected", Configured: true},
		{Name: "OpenSubtitles", State: "limited", Configured: true, LastSafeError: subtitleDailyLimitError},
		{Name: "SubSource", State: "unavailable", Configured: true},
		{Name: "Other", State: "untested", Configured: true},
		{Name: "Last", State: "not configured"},
	}})
	for _, fragment := range []string{
		`class="subtitle-state ready">Connected<`, `class="subtitle-state limited">Limited<`, `class="subtitle-state unavailable">Unavailable<`,
		`class="subtitle-state pending">Not tested yet<`, `class="subtitle-state pending">Not configured<`,
	} {
		if !strings.Contains(page, fragment) {
			t.Errorf("provider list lacks %q", fragment)
		}
	}
	if strings.Contains(page, ">not configured<") || strings.Contains(page, ">connected<") {
		t.Error("provider list shows raw state values")
	}
}

func TestDashboardSaysNothingChangedWhenACheckHadNoWork(t *testing.T) {
	t.Parallel()
	manager := &subtitleManager{}
	manager.noteAutomation(subtitleMaintenanceResult{}, time.Date(2026, 10, 30, 22, 0, 0, 0, time.UTC))
	page := renderSummary(t, subtitleDashboardData{Total: 3, Automation: manager.automationStatus()})
	if !strings.Contains(page, "</time>: nothing changed. Next check") || strings.Contains(page, "0 added") {
		t.Fatal("an idle automatic check reads as a result with zero counts")
	}
}
