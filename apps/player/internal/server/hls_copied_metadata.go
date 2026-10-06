package server

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/workload"
)

type copiedHLSMetadata struct {
	mu        sync.Mutex
	gate      chan struct{}
	endpoints map[string]copiedHLSEndpoint
}

type copiedHLSEndpoint struct {
	policy   string
	manifest [32]byte
	assets   []os.FileInfo
	end      float64
}

func (manager *hlsManager) openCopiedHLSRoot(directory string) (*os.Root, error) {
	relative, err := filepath.Rel(manager.cache, directory)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return nil, errCopiedHLSIndex
	}
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	return root.OpenRoot(relative)
}

// One small one-thread metadata probe may accompany the existing owner encoder.
// A second encoding reservation here would deadlock a capacity-one Server.
func (manager *hlsManager) copiedHLSMetadataAdmission(parent context.Context) (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	if metadata.gate == nil {
		metadata.gate = make(chan struct{}, 1)
		metadata.endpoints = make(map[string]copiedHLSEndpoint)
	}
	gate := metadata.gate
	metadata.mu.Unlock()
	select {
	case gate <- struct{}{}:
		return ctx, func() { <-gate; cancel() }, nil
	case <-ctx.Done():
		cancel()
		return nil, nil, errCopiedHLSIndex
	}
}

func copiedHLSEndpointAssets(root *os.Root, manifest []byte) ([]os.FileInfo, error) {
	last := copiedHLSLastSegment(manifest)
	if last == "" {
		return nil, errCopiedHLSIndex
	}
	assets := make([]os.FileInfo, 0, 3)
	for _, name := range []string{"init.mp4", "segment-00000.m4s", last} {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 {
			return nil, errCopiedHLSIndex
		}
		assets = append(assets, info)
	}
	return assets, nil
}

func sameCopiedHLSAssets(first, second []os.FileInfo) bool {
	if len(first) != 3 || len(second) != 3 {
		return false
	}
	for number, info := range first {
		other := second[number]
		if !os.SameFile(info, other) || info.Size() != other.Size() || !info.ModTime().Equal(other.ModTime()) {
			return false
		}
	}
	return true
}

func (manager *hlsManager) cachedCopiedHLSEndpoint(directory, policy string, manifest []byte, assets []os.FileInfo) (float64, bool) {
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	value, ok := metadata.endpoints[directory]
	metadata.mu.Unlock()
	return value.end, ok && value.policy == policy && value.manifest == sha256.Sum256(manifest) && sameCopiedHLSAssets(value.assets, assets)
}

func (manager *hlsManager) copiedHLSEndpoint(ctx context.Context, directory, policy string, manifest []byte) (float64, error) {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return 0, errCopiedHLSIndex
	}
	defer root.Close()
	assets, err := copiedHLSEndpointAssets(root, manifest)
	if err != nil {
		return 0, err
	}
	if end, valid := manager.cachedCopiedHLSEndpoint(directory, policy, manifest, assets); valid {
		return end, nil
	}
	ctx, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	if end, valid := manager.cachedCopiedHLSEndpoint(directory, policy, manifest, assets); valid {
		return end, nil
	}
	releaseWork, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return 0, errCopiedHLSIndex
	}
	defer releaseWork()
	end, err := manager.completedCopiedHLSEndpoint(ctx, root, manifest)
	current, statErr := copiedHLSEndpointAssets(root, manifest)
	if err != nil || statErr != nil || !sameCopiedHLSAssets(assets, current) {
		return 0, errCopiedHLSIndex
	}
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	if len(metadata.endpoints) >= 64 {
		clear(metadata.endpoints)
	}
	metadata.endpoints[directory] = copiedHLSEndpoint{policy: policy, manifest: sha256.Sum256(manifest), assets: assets, end: end}
	metadata.mu.Unlock()
	return end, nil
}
