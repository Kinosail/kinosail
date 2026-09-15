package viewing

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestPreviewStoreBoundsWorkAndConsumesOnlySuccessfulApplies(t *testing.T) { //nolint:cyclop // One lifecycle test covers capacity, expiry, failure, and consumption.
	values := make(map[string]Preview)
	var mutex sync.Mutex
	var slots chan struct{}
	store := PreviewStore{Mutex: &mutex, Values: &values, Slots: &slots, Limit: 1, Workers: 1}
	release, err := store.Acquire(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Acquire(context.Background(), time.Now()); err == nil {
		t.Fatal("worker capacity allowed a second source operation")
	}
	release()

	preview := Preview{ID: "PREVIEW", ExpiresAt: time.Now().Add(time.Minute), Summary: Summary{Conflicts: 1}}
	if err := store.Add(preview, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(Preview{ID: "SECOND", ExpiresAt: time.Now().Add(time.Minute)}, time.Now()); err == nil {
		t.Fatal("preview limit accepted a second preview")
	}
	result, err := ApplyPreview(store, "PREVIEW", func(Preview) (int, int, int, error) { return 0, 0, 0, errors.New("disk") })
	if err == nil || result.Conflicts != 1 {
		t.Fatalf("failed result = %#v, error = %v", result, err)
	}
	if _, found := store.Get("PREVIEW", time.Now()); !found {
		t.Fatal("failed apply consumed preview")
	}
	result, err = ApplyPreview(store, "PREVIEW", func(Preview) (int, int, int, error) { return 2, 1, 3, nil })
	if err != nil || result.Applied != 2 || result.Conflicts != 2 || result.ListsApplied != 3 {
		t.Fatalf("applied result = %#v, error = %v", result, err)
	}
	if _, found := store.Get("PREVIEW", time.Now()); found {
		t.Fatal("successful apply kept preview")
	}
	if _, err := ApplyPreview(store, "MISSING", func(Preview) (int, int, int, error) { return 0, 0, 0, nil }); err == nil {
		t.Fatal("missing preview was accepted")
	}

	expired := Preview{ID: "OLD", ExpiresAt: time.Now().Add(-time.Second)}
	values[expired.ID] = expired
	store.ExpireID(expired.ID, expired.ExpiresAt.Add(time.Second), time.Now())
	if _, found := values[expired.ID]; !found {
		t.Fatal("different preview generation was expired")
	}
	store.ExpireID(expired.ID, expired.ExpiresAt, time.Now())
	if _, found := values[expired.ID]; found {
		t.Fatal("expired preview remained")
	}
}

func TestPrepareStopsBeforeLaterSideEffects(t *testing.T) { //nolint:cyclop // One test proves each failed stage prevents all later work.
	values := make(map[string]Preview)
	var mutex sync.Mutex
	var slots chan struct{}
	store := PreviewStore{Mutex: &mutex, Values: &values, Slots: &slots, Limit: 2, Workers: 1}
	snapshots, builds := 0, 0
	_, err := Prepare(context.Background(), store, func(context.Context) ([]Activity, error) { return nil, errors.New("source") }, func() ([]library.Item, error) { snapshots++; return nil, nil }, func([]Activity, []library.Item) Preview { builds++; return Preview{} })
	if err == nil || snapshots != 0 || builds != 0 || len(values) != 0 {
		t.Fatalf("fetch failure side effects: snapshots=%d builds=%d previews=%d", snapshots, builds, len(values))
	}
	_, err = Prepare(context.Background(), store, func(context.Context) ([]Activity, error) { return []Activity{}, nil }, func() ([]library.Item, error) { snapshots++; return nil, errors.New("library") }, func([]Activity, []library.Item) Preview { builds++; return Preview{} })
	if err == nil || snapshots != 1 || builds != 0 || len(values) != 0 {
		t.Fatalf("snapshot failure side effects: snapshots=%d builds=%d previews=%d", snapshots, builds, len(values))
	}
	preview, err := Prepare(context.Background(), store, func(context.Context) ([]Activity, error) { return []Activity{}, nil }, func() ([]library.Item, error) { return []library.Item{}, nil }, func([]Activity, []library.Item) Preview {
		return Preview{ID: "READY", ExpiresAt: time.Now().Add(time.Minute)}
	})
	if err != nil || preview.ID != "READY" || len(values) != 1 {
		t.Fatalf("preview = %#v, error = %v", preview, err)
	}
}

func TestPreviewStoreCoversFullCanceledAndTimerPaths(t *testing.T) {
	values := map[string]Preview{"full": {ID: "full", ExpiresAt: time.Now().Add(time.Hour)}}
	var mutex sync.Mutex
	var slots chan struct{}
	store := PreviewStore{Mutex: &mutex, Values: &values, Slots: &slots, Limit: 1, Workers: 1}
	if _, err := store.Acquire(t.Context(), time.Now()); err == nil {
		t.Fatal("full preview store accepted work")
	}
	delete(values, "full")
	slots = make(chan struct{}, 1)
	slots <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Acquire(ctx, time.Now()); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("canceled acquire error=%v", err)
	}
	<-slots
	if err := store.Add(Preview{ID: "timer", ExpiresAt: time.Now().Add(time.Millisecond)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	expired := false
	for deadline := time.Now().Add(time.Second); !expired && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		mutex.Lock()
		expired = len(values) == 0
		mutex.Unlock()
	}
	if !expired {
		t.Fatal("timer did not expire preview")
	}

	mutex.Lock()
	values = map[string]Preview{"full": {ID: "full", ExpiresAt: time.Now().Add(time.Hour)}}
	mutex.Unlock()
	if _, err := Prepare(t.Context(), store, func(context.Context) ([]Activity, error) { return nil, nil }, func() ([]library.Item, error) { return nil, nil }, func([]Activity, []library.Item) Preview { return Preview{} }); err == nil {
		t.Fatal("prepare bypassed full store")
	}
	mutex.Lock()
	values = make(map[string]Preview)
	mutex.Unlock()
	_, err := Prepare(t.Context(), store, func(context.Context) ([]Activity, error) { return nil, nil }, func() ([]library.Item, error) {
		mutex.Lock()
		values["raced"] = Preview{ID: "raced", ExpiresAt: time.Now().Add(time.Hour)}
		mutex.Unlock()
		return nil, nil
	}, func([]Activity, []library.Item) Preview {
		return Preview{ID: "new", ExpiresAt: time.Now().Add(time.Hour)}
	})
	if err == nil {
		t.Fatal("prepare accepted a full store after snapshot")
	}
}
