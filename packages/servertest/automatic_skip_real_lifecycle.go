package servertest

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/playback"
)

func realHLSLifecycle(t *testing.T, cache string) context.Context {
	t.Helper()
	lifecycle, cancel := context.WithCancel(context.WithoutCancel(t.Context()))
	t.Cleanup(func() {
		defer cancel()
		ctx, stop := context.WithTimeout(lifecycle, 45*time.Second)
		defer stop()
		if !playback.WaitHLSReady(ctx, func() error { return finalizedRealHLSCache(cache) }) {
			t.Error("real HLS encoder did not finalize its cached variants before cleanup")
		}
	})
	return lifecycle
}

func finalizedRealHLSCache(cache string) error {
	return filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "index.m3u8" || !playback.QualityDirectory(filepath.Base(filepath.Dir(path))) {
			return nil
		}
		if !playback.FinalizedVariant(filepath.Dir(path)) {
			return errors.Join(os.ErrNotExist, errors.New("variant is still encoding"))
		}
		return nil
	})
}
