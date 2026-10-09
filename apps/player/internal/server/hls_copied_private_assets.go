package server

import (
	"context"
	"os"
	"path/filepath"
)

// These retained handles describe a joined private-stage snapshot. They do not
// own its producing worker or authorize certificates, publication or readiness.
type copiedHLSPrivateAssets struct {
	cacheDirectory string
	relative       string
	cache          *os.Root
	root           *os.Root
	cacheInfo      os.FileInfo
	rootInfo       os.FileInfo
	files          [2]*os.File
	info           [2]os.FileInfo
}

func (manager *hlsManager) openCopiedHLSPrivateAssets(ctx context.Context, directory string) (*copiedHLSPrivateAssets, error) {
	if ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	relative, err := filepath.Rel(manager.cache, directory)
	if err != nil || !filepath.IsLocal(relative) || relative == "." || filepath.Base(relative) != relative {
		return nil, errCopiedHLSIndex
	}
	assets := &copiedHLSPrivateAssets{cacheDirectory: manager.cache, relative: relative}
	if err := assets.openRoots(); err != nil {
		assets.close()
		return nil, err
	}
	limits := [2]int64{2 << 20, 64 << 20}
	for number, name := range []string{"init.mp4", "segment-00000.m4s"} {
		assets.files[number], assets.info[number], err = copiedHLSOpenFile(assets.root, name, limits[number])
		if err != nil {
			assets.close()
			return nil, errCopiedHLSIndex
		}
	}
	if !assets.bound(ctx) {
		assets.close()
		return nil, errCopiedHLSIndex
	}
	return assets, nil
}

func (assets *copiedHLSPrivateAssets) openRoots() error {
	var err error
	assets.cacheInfo, err = os.Lstat(assets.cacheDirectory)
	if err != nil || !assets.cacheInfo.IsDir() {
		return errCopiedHLSIndex
	}
	assets.cache, err = os.OpenRoot(assets.cacheDirectory)
	if err != nil {
		return errCopiedHLSIndex
	}
	assets.rootInfo, err = assets.cache.Lstat(assets.relative)
	if err != nil || !assets.rootInfo.IsDir() {
		return errCopiedHLSIndex
	}
	assets.root, err = assets.cache.OpenRoot(assets.relative)
	if err != nil {
		return errCopiedHLSIndex
	}
	return nil
}

func (assets *copiedHLSPrivateAssets) bound(ctx context.Context) bool {
	if !assets.currentRoots(ctx) {
		return false
	}
	for number, name := range []string{"init.mp4", "segment-00000.m4s"} {
		if assets.files[number] == nil {
			return false
		}
		opened, err := assets.files[number].Stat()
		current, currentErr := assets.root.Lstat(name)
		if err != nil || currentErr != nil || !sameCopiedHLSFile(assets.info[number], opened) ||
			!sameCopiedHLSFile(assets.info[number], current) {
			return false
		}
	}
	return ctx.Err() == nil
}

func (assets *copiedHLSPrivateAssets) currentRoots(ctx context.Context) bool {
	if ctx.Err() != nil || assets.cache == nil || assets.root == nil {
		return false
	}
	current, err := os.Lstat(assets.cacheDirectory)
	if !sameCopiedHLSPrivateRoot(assets.cache, assets.cacheInfo, current, err) {
		return false
	}
	current, err = assets.cache.Lstat(assets.relative)
	return sameCopiedHLSPrivateRoot(assets.root, assets.rootInfo, current, err)
}

func sameCopiedHLSPrivateRoot(root *os.Root, before, current os.FileInfo, currentErr error) bool {
	if root == nil || before == nil || currentErr != nil || current == nil || !current.IsDir() {
		return false
	}
	opened, err := root.Stat(".")
	return err == nil && os.SameFile(before, opened) && os.SameFile(opened, current)
}

func (assets *copiedHLSPrivateAssets) close() {
	for number, file := range assets.files {
		if file != nil {
			_ = file.Close()
			assets.files[number] = nil
		}
	}
	if assets.root != nil {
		_ = assets.root.Close()
		assets.root = nil
	}
	if assets.cache != nil {
		_ = assets.cache.Close()
		assets.cache = nil
	}
}
