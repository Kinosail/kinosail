package maintenance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestConcurrentMediaReadsPreventAllUpkeep(t *testing.T) {
	const readers = 24
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	state := &fixture{}
	manager := New(canceledContext(t), time.Hour, 1, state.dependencies())
	manager.now = func() time.Time { return time.Unix(0, 100) }
	entered := make(chan struct{}, readers)
	release := make(chan struct{})
	tracked := manager.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		entered <- struct{}{}
		<-release
	}))

	var group sync.WaitGroup
	releaseReaders := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseReaders()
		group.Wait()
	})
	for range readers {
		group.Add(1)
		go func() {
			defer group.Done()
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/media/item", nil)
			tracked.ServeHTTP(httptest.NewRecorder(), request)
		}()
	}
	for range readers {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("concurrent media reads did not reach the handler")
		}
	}
	if manager.foreground.Load() != readers || !manager.Busy() {
		t.Fatalf("active reads: foreground=%d busy=%t", manager.foreground.Load(), manager.Busy())
	}
	if err := manager.Run(); err != nil || state.prunes.Load() != 0 || state.metadataPrunes.Load() != 0 {
		t.Fatalf("busy upkeep: cache=%d metadata=%d error=%v", state.prunes.Load(), state.metadataPrunes.Load(), err)
	}

	releaseReaders()
	group.Wait()
	if manager.foreground.Load() != 0 || !manager.Busy() {
		t.Fatalf("quiet period: foreground=%d busy=%t", manager.foreground.Load(), manager.Busy())
	}
	manager.quietUntil.Store(manager.now().UnixNano())
	if manager.Busy() {
		t.Fatal("the exact concurrent quiet boundary must be idle")
	}
}
