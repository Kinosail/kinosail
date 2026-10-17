package catalog_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func blockedFirstScan(ctx context.Context, root string, wait time.Duration) (*catalog.Index, chan struct{}, time.Duration) {
	release := make(chan struct{})
	started := time.Now()
	index := catalog.NewDecoratedIndex(ctx, []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, func(ctx context.Context) (func(), error) {
		select {
		case <-release:
			return func() {}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, catalog.Decoration{StartupWait: wait})
	return index, release, time.Since(started)
}

func TestStartupReturnsWhileTheFirstLibraryScanIsBlocked(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, filepath.Join(root, "Film (2020).mp4"))
	index, release, elapsed := blockedFirstScan(t.Context(), root, 50*time.Millisecond)
	if elapsed > 2*time.Second {
		t.Fatalf("startup waited %s for a blocked Library scan", elapsed)
	}
	snapshot, err := index.Library()
	if !errors.Is(err, library.ErrLoading) || snapshot.Len() != 0 {
		t.Fatalf("library during blocked scan = %d items, %v", snapshot.Len(), err)
	}
	if _, _, statusErr := index.Status(); !errors.Is(statusErr, library.ErrLoading) {
		t.Fatalf("status during blocked scan = %v", statusErr)
	}
	close(release)
	eventually(t, "first Library scan was not published", func() bool {
		snapshot, err := index.Library()
		return err == nil && snapshot.Len() == 1
	})
	if _, _, statusErr := index.Status(); statusErr != nil {
		t.Fatalf("status after first scan = %v", statusErr)
	}
}

func TestStartupWaitsForAFastFirstLibraryScan(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, filepath.Join(root, "Film (2020).mp4"))
	index := catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, nil, catalog.Decoration{StartupWait: time.Minute})
	if snapshot, err := index.Library(); err != nil || snapshot.Len() != 1 {
		t.Fatalf("fast first scan = %d items, %v", snapshot.Len(), err)
	}
}

func TestStartupReportsAFirstScanThatCannotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	index, _, _ := blockedFirstScan(ctx, t.TempDir(), time.Millisecond)
	cancel()
	eventually(t, "cancelled first scan still reported loading", func() bool {
		_, err := index.Library()
		_, _, cause := index.Status()
		return errors.Is(err, library.ErrUnavailable) && errors.Is(cause, context.Canceled)
	})
}
