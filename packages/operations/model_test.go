package operations

import (
	"testing"
	"time"
)

func TestBuildDiagnosticReportUsesSafeSnapshot(t *testing.T) { //nolint:cyclop // One comparison set protects every safe report field.
	t.Parallel()
	started := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.FixedZone("MDT", -6*60*60))
	now := started.Add(95*time.Second + 900*time.Millisecond)
	snapshot := DiagnosticSnapshot{
		LastScan: time.Date(2026, time.September, 4, 17, 30, 0, 0, time.UTC), PlaybackMode: "direct", Transcoder: "fast", ScanFrequency: "15m",
		LibraryMonitoring: true, Healthy: true, ScanError: true, ActivityHealthy: true, LibraryItems: 19, Sessions: 3, TranscodeCacheBytes: 42,
		HTTPRequests: 51, HTTPErrors: 2, HTTPPanics: 1, ActivityFailures: 4,
	}
	report := BuildDiagnosticReport("v1.2.3", started, now, snapshot)
	if report.Version != "v1.2.3" || report.Generated != "2026-09-04T18:01:35Z" || report.UptimeSeconds != 95 || report.LastScan != "2026-09-04T17:30:00Z" {
		t.Fatalf("identity and time = %#v", report)
	}
	if report.PlaybackMode != "direct" || report.Transcoder != "fast" || report.ScanFrequency != "15m" || report.LibraryMonitoring != "watching" {
		t.Fatalf("settings = %#v", report)
	}
	if !report.Healthy || !report.ScanError || !report.ActivityHealthy || report.LibraryItems != 19 || report.Sessions != 3 || report.TranscodeCacheBytes != 42 {
		t.Fatalf("health = %#v", report)
	}
	if report.HTTPRequests != 51 || report.HTTPErrors != 2 || report.HTTPPanics != 1 || report.ActivityFailures != 4 {
		t.Fatalf("activity = %#v", report)
	}
}

func TestBuildDiagnosticReportReportsPolling(t *testing.T) {
	t.Parallel()
	if report := BuildDiagnosticReport("dev", time.Time{}, time.Time{}, DiagnosticSnapshot{}); report.LibraryMonitoring != "polling" {
		t.Fatalf("monitoring = %q", report.LibraryMonitoring)
	}
}
