package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSyncRejectsCapacityMissingAndConcurrentRuns(t *testing.T) {
	manager := newTestManager("", testManagerConfig())
	manager.syncs["sync"] = Sync{ID: "sync", Source: "plex", URL: "https://source.example", Token: "secret", ProfileID: "viewer", Interval: "1h"}
	manager.syncSlots <- struct{}{}
	manager.syncSlots <- struct{}{}
	if _, err := manager.RunSync(t.Context(), "sync"); err == nil {
		t.Fatal("capacity bypassed")
	}
	<-manager.syncSlots
	<-manager.syncSlots
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	manager.syncSlots <- struct{}{}
	manager.syncSlots <- struct{}{}
	if _, err := manager.RunSync(ctx, "sync"); err == nil {
		t.Fatal("canceled run accepted")
	}
	<-manager.syncSlots
	<-manager.syncSlots
	if _, err := manager.runReserved(t.Context(), "missing"); err == nil {
		t.Fatal("missing sync accepted")
	}
	sync := manager.syncs["sync"]
	sync.Running = true
	manager.syncs["sync"] = sync
	if _, err := manager.runReserved(t.Context(), "sync"); err == nil {
		t.Fatal("concurrent run accepted")
	}
}

func TestManagerViewsAndRemovesSyncsWithoutSecrets(t *testing.T) { //nolint:cyclop // One lifecycle assertion covers every credential-redaction boundary.
	manager := newTestManager(t.TempDir(), testManagerConfig())
	now := time.Now()
	manager.syncs["b"] = Sync{ID: "b", Token: "secret", ProfileID: "viewer", Interval: "1h", LastRun: now, NextRun: now.Add(time.Hour), Running: true}
	manager.syncs["a"] = Sync{ID: "a", Token: "secret", ProfileID: "viewer", Interval: "1h"}
	views := manager.Syncs()
	if len(views) != 2 || views[0].ID != "a" || views[1].LastRun == "" || views[1].NextRun == "" || !views[1].Running {
		t.Fatalf("views=%#v", views)
	}
	if err := manager.RemoveSync("missing"); err == nil {
		t.Fatal("missing removal accepted")
	}
	if err := manager.RemoveSync("a"); err != nil || len(manager.syncs) != 1 {
		t.Fatalf("remove error=%v state=%#v", err, manager.syncs)
	}
	manager.config.Persist = func(string, any) error { return errors.New("disk") }
	if err := manager.RemoveSync("b"); err == nil || len(manager.syncs) != 1 {
		t.Fatalf("failed remove state=%#v error=%v", manager.syncs, err)
	}
}

func TestManagerSaveFailsClosed(t *testing.T) {
	manager := newTestManager("", testManagerConfig())
	if err := manager.save(nil); err == nil {
		t.Fatal("empty storage accepted")
	}
	manager.file = "state.json"
	manager.config.Persist = nil
	if err := manager.save(nil); err == nil {
		t.Fatal("nil persistence accepted")
	}
	manager.loadErr = errors.New("invalid state")
	if err := manager.save(nil); err == nil {
		t.Fatal("invalid restored state was overwritten")
	}
}

func TestDueSyncWorkersAreBounded(t *testing.T) {
	var active, maximum atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		<-release
		active.Add(-1)
		_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[]}}`))
	}))
	defer server.Close()
	config := testManagerConfig()
	config.Client = server.Client()
	manager := newTestManager(t.TempDir(), config)
	for _, id := range []string{"a", "b", "c", "d"} {
		manager.syncs[id] = Sync{ID: id, Source: "plex", URL: server.URL, Token: "secret", ProfileID: "viewer", Interval: "1h", NextRun: time.Now().Add(-time.Hour)}
	}
	manager.RunDue(t.Context(), time.Now())
	for deadline := time.Now().Add(time.Second); maximum.Load() < syncWorkers && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
	}
	if maximum.Load() != syncWorkers || manager.syncDeferred.Load() != 2 {
		close(release)
		t.Fatalf("workers=%d deferred=%d", maximum.Load(), manager.syncDeferred.Load())
	}
	close(release)
}
