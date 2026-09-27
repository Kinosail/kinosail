// Package operations owns Kinosail owner tasks, metrics, diagnostics, and metadata refresh coordination.
package operations

import (
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/liveevents"
	"github.com/MikeO7/kinosail/packages/watchrooms"
)

// WorkloadSnapshot contains scheduler capacity and pressure.
type WorkloadSnapshot struct {
	Capacity, BackgroundCapacity                                         int
	ActivePlayback, ActiveBackground, WaitingPlayback, WaitingBackground int64
}

type runtimeSnapshot struct {
	LastScan                                time.Time
	PlaybackMode, Transcoder, ScanFrequency string
	LibraryMonitoring                       bool
	ScanError, CacheError                   error
	LibraryItems, Sessions                  int
	TranscodeCacheBytes                     int64
	HTTPRequests, HTTPErrors, HTTPPanics    uint64
	Activity                                auditjournal.Status
	Workload                                WorkloadSnapshot
	ViewingSyncActive                       int
	ViewingSyncDeferred                     uint64
	LiveEvents                              liveevents.Metrics
	WatchRooms                              watchrooms.Metrics
}

// Metrics is one safe Prometheus snapshot of server activity.
type Metrics struct {
	Healthy, LibraryMonitoring                                                          bool
	LibraryItems, Sessions                                                              int
	TranscodeCacheBytes                                                                 int64
	HTTPRequests, HTTPErrors, HTTPPanics, ActivityFailures                              uint64
	WorkloadCapacity, WorkloadBackgroundCapacity                                        int
	ActivePlayback, ActiveBackground, WaitingPlayback, WaitingBackground                int64
	NotificationQueueDepth                                                              int
	NotificationDropped, ViewingSyncDeferred                                            uint64
	ViewingSyncActive                                                                   int
	LiveEventSubscribers                                                                int
	LiveEventsPublished, LiveEventReconnects, LiveEventRejected, LiveEventSlowDrops     uint64
	WatchRoomConnections                                                                int
	WatchRoomJoins, WatchRoomReconnects, WatchRoomSlowDrops, WatchRoomDriftObservations uint64
	WatchRoomDriftSeconds                                                               float64
}

func projectMetrics(snapshot runtimeSnapshot) Metrics {
	workload, activity, live, rooms := snapshot.Workload, snapshot.Activity, snapshot.LiveEvents, snapshot.WatchRooms
	return Metrics{
		Healthy: activity.Healthy && snapshot.ScanError == nil, LibraryMonitoring: snapshot.LibraryMonitoring, LibraryItems: snapshot.LibraryItems, Sessions: snapshot.Sessions,
		TranscodeCacheBytes: snapshot.TranscodeCacheBytes, HTTPRequests: snapshot.HTTPRequests, HTTPErrors: snapshot.HTTPErrors, HTTPPanics: snapshot.HTTPPanics, ActivityFailures: activity.WriteFailures,
		WorkloadCapacity: workload.Capacity, WorkloadBackgroundCapacity: workload.BackgroundCapacity, ActivePlayback: workload.ActivePlayback, ActiveBackground: workload.ActiveBackground, WaitingPlayback: workload.WaitingPlayback, WaitingBackground: workload.WaitingBackground,
		NotificationQueueDepth: activity.NotificationQueued, NotificationDropped: activity.NotificationDrops, ViewingSyncActive: snapshot.ViewingSyncActive, ViewingSyncDeferred: snapshot.ViewingSyncDeferred,
		LiveEventSubscribers: live.Subscribers, LiveEventsPublished: live.Published, LiveEventReconnects: live.Reconnects, LiveEventRejected: live.Rejected, LiveEventSlowDrops: live.SlowDrops,
		WatchRoomConnections: rooms.Connections, WatchRoomJoins: rooms.Joins, WatchRoomReconnects: rooms.Reconnects, WatchRoomSlowDrops: rooms.SlowDrops, WatchRoomDriftObservations: rooms.DriftObservations, WatchRoomDriftSeconds: rooms.DriftSeconds,
	}
}

// DiagnosticSnapshot contains the current values used to build a safe report.
type DiagnosticSnapshot struct {
	LastScan                                               time.Time
	PlaybackMode, Transcoder, ScanFrequency                string
	LibraryMonitoring, Healthy, ScanError, ActivityHealthy bool
	LibraryItems, Sessions                                 int
	TranscodeCacheBytes                                    int64
	HTTPRequests, HTTPErrors, HTTPPanics, ActivityFailures uint64
}

// DiagnosticReport is the safe downloadable server diagnostic document.
type DiagnosticReport struct {
	Version             string         `json:"version"`
	Generated           string         `json:"generated"`
	UptimeSeconds       int64          `json:"uptimeSeconds"`
	LastScan            string         `json:"lastScan"`
	PlaybackMode        string         `json:"playbackMode"`
	Transcoder          string         `json:"transcoder"`
	ScanFrequency       string         `json:"scanFrequency"`
	LibraryMonitoring   string         `json:"libraryMonitoring"`
	Healthy             bool           `json:"healthy"`
	ScanError           bool           `json:"scanError"`
	LibraryItems        int            `json:"libraryItems"`
	Sessions            int            `json:"sessions"`
	TranscodeCacheBytes int64          `json:"transcodeCacheBytes"`
	HTTPRequests        uint64         `json:"httpRequests"`
	HTTPErrors          uint64         `json:"httpErrors"`
	HTTPPanics          uint64         `json:"httpPanics"`
	ActivityFailures    uint64         `json:"activityFailures"`
	ActivityHealthy     bool           `json:"activityHealthy"`
	RecentFailures      []FailureEvent `json:"recentFailures,omitempty"`
}

// BuildDiagnosticReport adds process identity and time values to a diagnostic snapshot.
func BuildDiagnosticReport(version string, started, now time.Time, snapshot DiagnosticSnapshot) DiagnosticReport {
	monitoring := "polling"
	if snapshot.LibraryMonitoring {
		monitoring = "watching"
	}
	return DiagnosticReport{
		version, now.UTC().Format(time.RFC3339), int64(now.Sub(started).Seconds()), snapshot.LastScan.UTC().Format(time.RFC3339),
		snapshot.PlaybackMode, snapshot.Transcoder, snapshot.ScanFrequency, monitoring, snapshot.Healthy, snapshot.ScanError,
		snapshot.LibraryItems, snapshot.Sessions, snapshot.TranscodeCacheBytes, snapshot.HTTPRequests, snapshot.HTTPErrors,
		snapshot.HTTPPanics, snapshot.ActivityFailures, snapshot.ActivityHealthy, nil,
	}
}

func projectDiagnosticReport(version string, started, now time.Time, snapshot runtimeSnapshot) DiagnosticReport {
	return BuildDiagnosticReport(version, started, now, DiagnosticSnapshot{
		LastScan: snapshot.LastScan, PlaybackMode: snapshot.PlaybackMode, Transcoder: snapshot.Transcoder, ScanFrequency: snapshot.ScanFrequency, LibraryMonitoring: snapshot.LibraryMonitoring,
		Healthy: snapshot.ScanError == nil && snapshot.CacheError == nil && snapshot.Activity.Healthy, ScanError: snapshot.ScanError != nil, LibraryItems: snapshot.LibraryItems, Sessions: snapshot.Sessions, TranscodeCacheBytes: snapshot.TranscodeCacheBytes,
		HTTPRequests: snapshot.HTTPRequests, HTTPErrors: snapshot.HTTPErrors, HTTPPanics: snapshot.HTTPPanics, ActivityFailures: snapshot.Activity.WriteFailures, ActivityHealthy: snapshot.Activity.Healthy,
	})
}
