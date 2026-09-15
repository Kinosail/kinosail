package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexUpdateAndBackgroundFailureEdges(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Movie.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewMemoryIndex(nil, false)
	if err := index.UpdateRoots(t.Context(), []ScanRoot{{Path: directory, Namespace: "Movies"}}); err != nil {
		t.Fatal(err)
	}
	if items, err := index.Snapshot(); err != nil || len(items) != 1 {
		t.Fatalf("updated index = %#v, %v", items, err)
	}

	wantErr := errors.New("busy")
	called := make(chan struct{})
	index.acquire = func(context.Context) (func(), error) {
		close(called)
		return nil, wantErr
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	index.RequestRefresh()
	index.RequestRefresh()
	go index.runRefreshes(ctx)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not run")
	}
}
