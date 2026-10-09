package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

type copiedAACSlowWriter struct {
	*httptest.ResponseRecorder
	entered, release chan struct{}
}

func (writer *copiedAACSlowWriter) Write(data []byte) (int, error) {
	select {
	case writer.entered <- struct{}{}:
	default:
	}
	<-writer.release
	return writer.ResponseRecorder.Write(data)
}

// Gap: genuine HTTP cannot reliably hold exactly after metadata admission.
func TestCopiedAACSlowNetworkDoesNotRetainMetadataLease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, directory, held := copiedAACGenerationFixture(t)
	item, recipe := held.item, held.recipe
	held.close()
	owner := newCopiedAACDelivery(t)
	request := httptest.NewRequestWithContext(owner.ctx, http.MethodGet, "/init.mp4", nil)
	go func() {
		owner.done <- manager.serveCopiedAACFile(owner.writer, request, item, recipe, "360p/init.mp4", filepath.Base(directory))
	}()
	owner.waitEntered(t, "controlled asset did not reach retained-byte delivery")
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		t.Fatal("slow network retained the metadata gate after proof completed")
	}
	release()
	owner.release()
	owner.join(t, "retained asset delivery failed after independent admission")
}

type copiedAACDeliveryOwner struct {
	ctx    context.Context
	cancel context.CancelFunc
	writer *copiedAACSlowWriter
	done   chan bool
	joined bool
	once   sync.Once
}

func newCopiedAACDelivery(t *testing.T) *copiedAACDeliveryOwner {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	owner := &copiedAACDeliveryOwner{
		ctx: ctx, cancel: cancel, done: make(chan bool, 1),
		writer: &copiedAACSlowWriter{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}, 1), release: make(chan struct{})},
	}
	t.Cleanup(func() {
		owner.cancel()
		owner.release()
		if !owner.joined {
			select {
			case <-owner.done:
				owner.joined = true
			case <-time.After(3 * time.Second):
				t.Error("owned retained-byte caller did not join during cleanup")
			}
		}
	})
	return owner
}

func (owner *copiedAACDeliveryOwner) release() {
	owner.once.Do(func() { close(owner.writer.release) })
}

func (owner *copiedAACDeliveryOwner) waitEntered(t *testing.T, failure string) {
	t.Helper()
	select {
	case <-owner.writer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal(failure)
	}
}

func (owner *copiedAACDeliveryOwner) join(t *testing.T, failure string) {
	t.Helper()
	select {
	case handled := <-owner.done:
		owner.joined = true
		if !handled || owner.writer.Code != http.StatusOK || owner.writer.Body.Len() == 0 {
			t.Fatal(failure)
		}
	case <-time.After(time.Second):
		t.Fatal("owned retained-byte delivery did not join")
	}
}
