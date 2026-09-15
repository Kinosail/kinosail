package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatchEventsEdges(t *testing.T) { //nolint:cyclop,funlen,gocognit // Each table row drives one select result through the shared watcher loop.
	wantErr := errors.New("failed")
	event := fsnotify.Event{Name: "movie.mp4", Op: fsnotify.Write}
	tests := []struct {
		name        string
		prepare     func(context.CancelFunc, chan struct{}, chan fsnotify.Event, chan error, chan time.Time)
		record      func(fsnotify.Event) error
		settle      func() error
		wantErr     error
		cleared     bool
		continues   bool
		afterRecord bool
		afterSettle bool
	}{
		{name: "context", prepare: func(cancel context.CancelFunc, _ chan struct{}, _ chan fsnotify.Event, _ chan error, _ chan time.Time) {
			cancel()
		}},
		{name: "change", prepare: func(_ context.CancelFunc, changes chan struct{}, _ chan fsnotify.Event, _ chan error, _ chan time.Time) {
			changes <- struct{}{}
		}},
		{name: "closed events", prepare: func(_ context.CancelFunc, _ chan struct{}, events chan fsnotify.Event, _ chan error, _ chan time.Time) {
			close(events)
		}, wantErr: errors.New("filesystem event stream closed")},
		{name: "record error", prepare: func(_ context.CancelFunc, _ chan struct{}, events chan fsnotify.Event, _ chan error, _ chan time.Time) {
			events <- event
		}, record: func(fsnotify.Event) error { return wantErr }, wantErr: wantErr},
		{name: "closed failures", prepare: func(_ context.CancelFunc, _ chan struct{}, _ chan fsnotify.Event, failures chan error, _ chan time.Time) {
			close(failures)
		}, wantErr: errors.New("filesystem error stream closed")},
		{name: "watcher failure", prepare: func(_ context.CancelFunc, _ chan struct{}, _ chan fsnotify.Event, failures chan error, _ chan time.Time) {
			failures <- wantErr
		}, wantErr: wantErr},
		{name: "settle error", prepare: func(_ context.CancelFunc, _ chan struct{}, _ chan fsnotify.Event, _ chan error, quiet chan time.Time) {
			quiet <- time.Now()
		}, settle: func() error { return wantErr }, wantErr: wantErr, cleared: true},
		{name: "settle and continue", prepare: func(_ context.CancelFunc, _ chan struct{}, _ chan fsnotify.Event, _ chan error, quiet chan time.Time) {
			quiet <- time.Now()
		}, cleared: true, continues: true, afterSettle: true},
		{name: "record and continue", prepare: func(_ context.CancelFunc, _ chan struct{}, events chan fsnotify.Event, _ chan error, _ chan time.Time) {
			events <- event
		}, continues: true, afterRecord: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			changes := make(chan struct{}, 1)
			events := make(chan fsnotify.Event, 1)
			failures := make(chan error, 1)
			quiet := make(chan time.Time, 1)
			cleared, recorded, settled := false, false, false
			record := test.record
			if record == nil {
				record = func(fsnotify.Event) error {
					recorded = true
					if test.afterRecord {
						changes <- struct{}{}
					}
					return nil
				}
			}
			settle := test.settle
			if settle == nil {
				settle = func() error {
					settled = true
					if test.afterSettle {
						changes <- struct{}{}
					}
					return nil
				}
			}
			test.prepare(cancel, changes, events, failures, quiet)
			err := watchEvents(ctx, changes, events, failures, func() <-chan time.Time { return quiet }, func() { cleared = true }, record, settle)
			if test.wantErr == nil && err != nil || test.wantErr != nil && (err == nil || err.Error() != test.wantErr.Error()) {
				t.Fatalf("watch error = %v, want %v", err, test.wantErr)
			}
			if cleared != test.cleared {
				t.Fatalf("cleared = %t, want %t", cleared, test.cleared)
			}
			if test.continues && !recorded && !settled {
				t.Fatal("watch event was not processed before continuing")
			}
		})
	}
}

func TestWatchFailureAndRetryEdges(t *testing.T) {
	index := NewMemoryIndex(nil, true)
	index.pollInterval = time.Hour
	index.watchRetry = time.Hour
	index.watchChange <- struct{}{}
	wantErr := errors.New("watch failed")
	calls := make(chan int, 2)
	call := 0
	index.watch = func(context.Context) error {
		call++
		calls <- call
		if call == 1 {
			return nil
		}
		return wantErr
	}
	ctx, cancel := context.WithCancel(t.Context())
	index.Watch(ctx)
	for expected := 1; expected <= 2; expected++ {
		select {
		case actual := <-calls:
			if actual != expected {
				t.Fatalf("watch call = %d, want %d", actual, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("watch retry did not run")
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		watching, err := index.Monitoring()
		if !watching && errors.Is(err, wantErr) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("monitoring state = %t, %v", watching, err)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
}

func TestPollErrorAndCancellationEdges(t *testing.T) { //nolint:funlen // The three subtests cover independent polling failure/cancellation phases.
	wantErr := errors.New("snapshot failed")
	t.Run("current snapshot error", func(t *testing.T) {
		index := NewMemoryIndex(nil, true)
		index.pollInterval = time.Millisecond
		calls := make(chan int, 3)
		count := 0
		index.snapshot = func(map[string]struct{}) (map[string]fileStamp, error) {
			count++
			calls <- count
			if count > 1 {
				return nil, wantErr
			}
			return nil, nil
		}
		cancel, done := runPoll(t, index)
		waitCall(t, calls, 2)
		cancel()
		<-done
	})
	t.Run("stability cancellation", func(t *testing.T) {
		index := NewMemoryIndex(nil, true)
		index.pollInterval, index.watchStability = time.Millisecond, time.Hour
		calls := make(chan int, 2)
		count := 0
		index.snapshot = func(map[string]struct{}) (map[string]fileStamp, error) {
			count++
			calls <- count
			return map[string]fileStamp{"movie": {size: int64(count)}}, nil
		}
		cancel, done := runPoll(t, index)
		waitCall(t, calls, 2)
		cancel()
		<-done
	})
	t.Run("stable snapshot error", func(t *testing.T) {
		index := NewMemoryIndex(nil, true)
		index.pollInterval, index.watchStability = time.Millisecond, time.Millisecond
		calls := make(chan int, 4)
		count := 0
		index.snapshot = func(map[string]struct{}) (map[string]fileStamp, error) {
			count++
			calls <- count
			if count == 3 {
				return nil, wantErr
			}
			return map[string]fileStamp{"movie": {size: int64(count)}}, nil
		}
		cancel, done := runPoll(t, index)
		waitCall(t, calls, 3)
		cancel()
		<-done
	})
}

func TestWatchChangesAndCycleEdges(t *testing.T) { //nolint:cyclop,funlen // Startup, filesystem, and settle errors share the watcher lifecycle.
	wantErr := errors.New("open failed")
	index := NewMemoryIndex(nil, true)
	index.openWatcher = func() (*fsnotify.Watcher, error) { return nil, wantErr }
	if err := index.watchChanges(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("open error = %v", err)
	}
	index = NewMemoryIndex(nil, true)
	index.SetRoots([]ScanRoot{{Path: string([]byte{0})}})
	if err := index.watchChanges(t.Context()); err == nil {
		t.Fatal("invalid watch root was accepted")
	}
	index = NewMemoryIndex(nil, true)
	index.watchChange <- struct{}{}
	if err := index.watchChanges(t.Context()); err != nil {
		t.Fatalf("watch change = %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	cycle := newWatchCycle()
	defer cycle.close()
	index = NewMemoryIndex(nil, true)
	index.watchDebounce, index.watchStability = time.Hour, time.Hour
	dispatch := watchDispatch{index: index, watcher: watcher, cycle: cycle}
	if dispatch.quiet() != nil {
		t.Fatal("new cycle unexpectedly armed")
	}
	dispatch.clearQuiet()
	if err := dispatch.record(fsnotify.Event{Name: string([]byte{0}), Op: fsnotify.Create}); err == nil {
		t.Fatal("invalid created path was accepted")
	}
	file := filepath.Join(t.TempDir(), "movie.mp4")
	if err := os.WriteFile(file, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := dispatch.record(fsnotify.Event{Name: file, Op: fsnotify.Write}); err != nil {
		t.Fatal(err)
	}
	if err := dispatch.settle(); err != nil {
		t.Fatal(err)
	}
	cycle.timer.Stop()
	if err := dispatch.settle(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-index.refreshes:
	default:
		t.Fatal("stable cycle did not request a refresh")
	}
	cycle.dirty = map[string]struct{}{string([]byte{0}): {}}
	if err := dispatch.settle(); err == nil {
		t.Fatal("invalid dirty path was accepted")
	}
}

func TestSnapshotAndWatchTreeEdges(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if state, err := snapshotFiles(map[string]struct{}{missing: {}}); err != nil || len(state) != 0 {
		t.Fatalf("missing snapshot = %#v, %v", state, err)
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if state, err := snapshotFiles(map[string]struct{}{link: {}}); err != nil || len(state) != 0 {
		t.Fatalf("symlink snapshot = %#v, %v", state, err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if err := addWatchTree(watcher, string([]byte{0})); err == nil {
		t.Fatal("invalid watch tree was accepted")
	}
}
