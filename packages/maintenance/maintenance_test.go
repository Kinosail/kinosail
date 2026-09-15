package maintenance

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct {
	prunes         atomic.Int64
	metadataPrunes atomic.Int64
	limit          atomic.Int64
	cacheErr       error
	metaErr        error
	backup         BackupStatus
	metadata       bool
	analysis       string
}

func (fixture *fixture) dependencies() Dependencies {
	return Dependencies{
		PruneCache: func(limit int64) (int64, error) {
			fixture.prunes.Add(1)
			fixture.limit.Store(limit)
			return 42, fixture.cacheErr
		},
		PruneMetadata: func() error {
			fixture.metadataPrunes.Add(1)
			return fixture.metaErr
		},
		CacheStats:   func() (int64, error) { return 42, fixture.cacheErr },
		BackupStatus: func() BackupStatus { return fixture.backup },
		MetadataConfigured: func() bool {
			return fixture.metadata
		},
		AnalysisStatus: func() string { return fixture.analysis },
	}
}

func canceledContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func TestManagerDefaultsAreBounded(t *testing.T) {
	manager := New(nil, 0, 0, (&fixture{}).dependencies()) //nolint:staticcheck // A nil context is the documented schedule-off mode.
	if manager.interval != 15*time.Minute || manager.limit != 10_737_418_240 {
		t.Fatalf("default interval and limit = %s and %d", manager.interval, manager.limit)
	}
	if initialDelay(time.Millisecond) != time.Millisecond || initialDelay(time.Second) != 250*time.Millisecond {
		t.Fatal("startup delay did not preserve the shorter bound")
	}
}

func TestManagerRunsBoundedUpkeep(t *testing.T) {
	state := &fixture{}
	manager := New(canceledContext(t), time.Hour, 0, state.dependencies())
	if err := manager.Run(); err != nil || state.prunes.Load() != 1 || state.metadataPrunes.Load() != 1 || state.limit.Load() != 10_737_418_240 {
		t.Fatalf("upkeep = %d cache and %d metadata calls at %d bytes: %v", state.prunes.Load(), state.metadataPrunes.Load(), state.limit.Load(), err)
	}
	state.cacheErr, state.metaErr = errors.New("cache failed"), errors.New("metadata failed")
	if err := manager.Run(); !errors.Is(err, state.cacheErr) || !errors.Is(err, state.metaErr) {
		t.Fatalf("joined upkeep failure = %v", err)
	}
	manager.foreground.Store(1)
	if err := manager.Run(); err != nil || state.prunes.Load() != 2 || state.metadataPrunes.Load() != 2 {
		t.Fatalf("busy upkeep = %d cache and %d metadata calls: %v", state.prunes.Load(), state.metadataPrunes.Load(), err)
	}
	manager.foreground.Store(0)
}

func TestManagerSchedulesUpkeep(t *testing.T) {
	state := &fixture{}
	manager := New(canceledContext(t), time.Hour, 1, state.dependencies())
	manager.schedule(canceledContext(t))
	if state.prunes.Load() != 0 || state.metadataPrunes.Load() != 0 {
		t.Fatalf("canceled schedule ran upkeep: cache=%d metadata=%d", state.prunes.Load(), state.metadataPrunes.Load())
	}
	scheduled, stop := context.WithCancel(t.Context())
	scheduledState := &fixture{}
	scheduledManager := New(canceledContext(t), time.Millisecond, 7, Dependencies{
		PruneCache: func(limit int64) (int64, error) {
			scheduledState.limit.Store(limit)
			stop()
			return 0, nil
		},
		PruneMetadata: func() error { return nil },
	})
	scheduledManager.schedule(scheduled)
	if scheduledState.limit.Load() != 7 {
		t.Fatalf("scheduled cache limit = %d", scheduledState.limit.Load())
	}
}

func TestManagerLogsUpkeepResult(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	state := &fixture{cacheErr: errors.New("cache failed")}
	manager := New(canceledContext(t), time.Hour, 1, state.dependencies())
	_ = manager.Run()
	if !bytes.Contains(output.Bytes(), []byte("automatic maintenance failed")) {
		t.Fatalf("failure log = %q", output.String())
	}
	output.Reset()
	state.cacheErr = nil
	_ = manager.Run()
	if !bytes.Contains(output.Bytes(), []byte("automatic maintenance completed")) {
		t.Fatalf("success log = %q", output.String())
	}
}

func TestManagerTracksOnlyInteractiveMediaReads(t *testing.T) { //nolint:cyclop // The table enumerates every protected public media namespace.
	manager := New(canceledContext(t), time.Hour, 1, (&fixture{}).dependencies())
	manager.now = func() time.Time { return time.Unix(0, 100) }
	manager.quietUntil.Store(100)
	if manager.Busy() {
		t.Fatal("the exact quiet boundary must be idle")
	}
	manager.quietUntil.Store(101)
	if !manager.Busy() {
		t.Fatal("a future quiet boundary must be busy")
	}
	manager.quietUntil.Store(0)
	for _, path := range []string{"/media/id", "/hls/id", "/download/id", "/Videos/id", "/Audio/id", "/api/v1/downloads/id/file", "/Items/id/Download", "/Items/id/File"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		manager.Track(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			if !manager.Busy() {
				t.Errorf("%s was not protected", path)
			}
			writer.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || !manager.Busy() {
			t.Fatalf("tracked %s = %d, busy=%t", path, response.Code, manager.Busy())
		}
		manager.quietUntil.Store(0)
	}
	for _, request := range []*http.Request{
		httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/media/id", nil),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items/id/Other", nil),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/downloads/id", nil),
	} {
		manager.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			if manager.Busy() {
				t.Errorf("unprotected %s was tracked", request.URL.Path)
			}
		})).ServeHTTP(httptest.NewRecorder(), request)
	}
}

func TestManagerWaitsForIdleOrCancellation(t *testing.T) {
	manager := New(canceledContext(t), time.Hour, 1, (&fixture{}).dependencies())
	idleContext, stopIdle := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer stopIdle()
	if err := manager.WaitIdle(idleContext); err != nil {
		t.Fatal(err)
	}
	manager.foreground.Store(1)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := manager.WaitIdle(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait = %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		manager.foreground.Store(0)
	}()
	if err := manager.WaitIdle(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestManagerStatusReportsEveryMaintenanceMode(t *testing.T) {
	state := &fixture{analysis: "queued"}
	manager := New(canceledContext(t), time.Hour, 9, state.dependencies())
	want := Status{Mode: "automatic", Activity: "idle", Attention: true, Library: Mode{"automatic"}, Metadata: Mode{"local"}, Analysis: AnalysisStatus{"on-change", "queued"}, Cache: CacheStatus{"bounded", 42, 9}, Backups: BackupStatus{State: "needs-setup"}}
	if got := manager.Status(); !reflect.DeepEqual(got, want) {
		t.Fatalf("setup status = %#v, want %#v", got, want)
	}
	state.backup = BackupStatus{Enabled: true, Encrypted: true}
	state.metadata = true
	manager.foreground.Store(1)
	want.Activity, want.Attention, want.Metadata.Mode = "busy", false, "automatic"
	want.Backups = BackupStatus{State: "automatic", Enabled: true, Encrypted: true}
	if got := manager.Status(); !reflect.DeepEqual(got, want) {
		t.Fatalf("automatic status = %#v, want %#v", got, want)
	}
	manager.foreground.Store(0)
	state.backup.LastError, state.cacheErr = "backup failed", errors.New("cache failed")
	want.Activity, want.Attention = "idle", true
	want.Cache.Bytes = 42
	want.Backups = BackupStatus{State: "error", Enabled: true, Encrypted: true, LastError: "backup failed"}
	if got := manager.Status(); !reflect.DeepEqual(got, want) {
		t.Fatalf("error status = %#v, want %#v", got, want)
	}
}
