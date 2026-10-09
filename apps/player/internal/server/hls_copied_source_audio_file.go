package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/MikeO7/kinosail/packages/library"
)

// Linux proc links resolve the retained file object, not its old pathname.
// Keep this read-only descriptor through both joined probes; no path fallback.
func openCopiedHLSSourceAudio(ctx context.Context, source string, before os.FileInfo) (*os.File, string, error) {
	if runtime.GOOS != "linux" || ctx.Err() != nil || before == nil || !before.Mode().IsRegular() || before.Size() <= 0 {
		return nil, "", errCopiedHLSIndex
	}
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return nil, "", errCopiedHLSIndex
	}
	defer root.Close()
	file, opened, err := copiedHLSOpenFile(root, filepath.Base(source), before.Size())
	if err != nil {
		return nil, "", errCopiedHLSIndex
	}
	if ctx.Err() != nil || !sameCopiedHLSFile(before, opened) {
		_ = file.Close()
		return nil, "", errCopiedHLSIndex
	}
	return file, fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), file.Fd()), nil
}

func (manager *hlsManager) copiedHLSSourceAudioComplete(ctx context.Context, item library.Item, recipe hlsRecipe, policy string, before os.FileInfo, file *os.File) bool {
	opened, err := file.Stat()
	if err != nil || !sameCopiedHLSFile(before, opened) {
		return false
	}
	_, err = manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, before)
	return err == nil && ctx.Err() == nil
}
