package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

const remainingAACReadAttempts = 5

func remainingAACStableRead[T any](read func() (T, error)) (T, error) {
	var value T
	var err error
	for range remainingAACReadAttempts {
		value, err = read()
		if !errors.Is(err, errCopiedHLSIndex) && !errors.Is(err, errHLSIdentityChanged) {
			return value, err
		}
	}
	return value, err
}

func (manager *hlsManager) remainingAACCacheAsset(ctx context.Context, item library.Item, recipe hlsRecipe, name string) error {
	if !remainingAACOriginRecipe(item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	duration := manager.probe.duration(ctx, item)
	if _, err := remainingAACStableRead(func() (struct{}, error) {
		if err := remainingAACReadyRoot(root, options.Cache); err != nil {
			return struct{}{}, err
		}
		if err := remainingAACManifestAsset(root, name, duration); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, remainingAACExistingAsset(root, name)
	}); err != nil {
		return err
	}
	return manager.validateHLSPolicy(ctx, item, recipe, options.Cache)
}

func (manager *hlsManager) openRemainingAACAsset(ctx context.Context, item library.Item, recipe hlsRecipe, name string) (*remainingAACAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !hlsFile(name) || filepath.Dir(name) != "audio" {
		return nil, errCopiedHLSIndex
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	root, err := manager.openCopiedHLSRoot(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe)))
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	duration := manager.probe.duration(ctx, item)
	return remainingAACStableRead(func() (*remainingAACAsset, error) {
		return manager.admittedRemainingAACAsset(ctx, item, recipe, root, name, options.Cache, duration)
	})
}

func (manager *hlsManager) admittedRemainingAACAsset(ctx context.Context, item library.Item, recipe hlsRecipe, root *os.Root, name, policy string, duration float64) (*remainingAACAsset, error) {
	if err := remainingAACReadyRoot(root, policy); err != nil {
		return nil, err
	}
	if err := remainingAACManifestAsset(root, name, duration); err != nil {
		return nil, err
	}
	file, info, err := copiedHLSOpenFile(root, name, 64<<20)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	asset := &remainingAACAsset{file: file, info: info, content: file}
	if err := manager.admitRemainingAACAsset(ctx, item, recipe, root, name, policy, asset); err != nil {
		_ = file.Close()
		return nil, err
	}
	return asset, nil
}
