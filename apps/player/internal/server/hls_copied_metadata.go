package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/workload"
)

type copiedHLSMetadata struct {
	mu          sync.Mutex
	gate        chan struct{}
	endpoints   map[string]copiedHLSEndpoint
	aacPolicies map[string]copiedAACPolicyDecision
}

type copiedHLSEndpoint struct {
	policy   string
	manifest [32]byte
	assets   []os.FileInfo
	end      float64
}

type copiedHLSMetadataKey struct{}

type copiedHLSClockCertificate struct {
	Version        int      `json:"version"`
	Rendition      string   `json:"rendition"`
	Timeline       [32]byte `json:"timeline"`
	Initialization [32]byte `json:"initialization"`
	First          [32]byte `json:"first"`
}

// Nonblocking open also rejects a regular-file-to-FIFO race without hanging.
func copiedHLSOpenFile(root *os.Root, name string, limit int64) (*os.File, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, nil, errCopiedHLSIndex
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	opened, err := file.Stat()
	if err != nil || !sameCopiedHLSFile(info, opened) {
		_ = file.Close()
		return nil, nil, errCopiedHLSIndex
	}
	return file, info, nil
}

func sameCopiedHLSFile(first, second os.FileInfo) bool {
	return first != nil && second != nil && second.Mode().IsRegular() && os.SameFile(first, second) && first.Size() == second.Size() && first.ModTime().Equal(second.ModTime())
}

type copiedHLSContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader copiedHLSContextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(data)
}

func copiedHLSAssetHash(ctx context.Context, root *os.Root, name string, limit int64) ([32]byte, error) {
	var result [32]byte
	file, before, err := copiedHLSOpenFile(root, name, limit)
	if err != nil {
		return result, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, copiedHLSContextReader{ctx, io.LimitReader(file, limit+1)})
	after, statErr := root.Lstat(name)
	if err != nil || ctx.Err() != nil || statErr != nil || size != before.Size() || !sameCopiedHLSFile(before, after) {
		return result, errCopiedHLSIndex
	}
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func (manager *hlsManager) verifyCopiedHLSCertificate(ctx context.Context, directory string, root *os.Root, data []byte, timeline *copiedHLSTimeline) error {
	if ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	certificateData, err := copiedHLSCacheFile(root, ".copy-clock", 4096)
	certificate, decodeErr := decodeCopiedHLSCertificate(certificateData, data)
	if err != nil || decodeErr != nil || certificate.Version != copiedHLSCertificateVersion(timeline) {
		return errCopiedHLSIndex
	}
	selected, err := copiedHLSRendition(root)
	if err != nil || selected != certificate.Rendition {
		return errCopiedHLSIndex
	}
	ctx, release, err := manager.copiedHLSClockAdmission(ctx)
	if err != nil {
		return err
	}
	defer release()
	rendition, err := root.OpenRoot(certificate.Rendition)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer rendition.Close()
	if err := verifyCopiedHLSRenditionAssets(ctx, rendition, certificate); err != nil {
		return err
	}
	if err := verifyCopiedAACAssets(ctx, rendition, timeline); err != nil {
		return err
	}
	return manager.verifyCopiedHLSBoundManifest(ctx, directory, root, rendition, data, certificateData, certificate.Rendition, timeline)
}

func verifyCopiedHLSRenditionAssets(ctx context.Context, rendition *os.Root, certificate copiedHLSClockCertificate) error {
	initialization, err := copiedHLSAssetHash(ctx, rendition, "init.mp4", 2<<20)
	if err != nil || initialization != certificate.Initialization {
		return errCopiedHLSIndex
	}
	if _, err := rendition.Lstat("segment-00000.m4s"); !os.IsNotExist(err) {
		first, err := copiedHLSAssetHash(ctx, rendition, "segment-00000.m4s", 64<<20)
		if err != nil || first != certificate.First {
			return errCopiedHLSIndex
		}
	} // Missing lazy media retains the certified init and cuts.
	return nil
}

func (manager *hlsManager) verifyCopiedHLSBoundManifest(ctx context.Context, directory string, root, rendition *os.Root, data, certificateData []byte, name string, timeline *copiedHLSTimeline) error {
	manifest, err := copiedHLSCacheFile(rendition, "index.m3u8", maximumCopiedHLSTimelineBytes)
	_, valid := copiedHLSManifest(manifest, timeline)
	if err != nil || !valid || !copiedHLSBoundMetadata(root, data, certificateData, timeline.Policy) || ctx.Err() != nil || !manager.copiedHLSCanonicalGeneration(directory, name, root, rendition) {
		return errCopiedHLSIndex
	}

	return nil
}

func decodeCopiedHLSCertificate(data, timeline []byte) (copiedHLSClockCertificate, error) {
	var certificate copiedHLSClockCertificate
	if httpguard.DecodeUniqueJSON(bytes.NewReader(data), 4096, &certificate) != nil || (certificate.Version != 1 && certificate.Version != 2) || !hlsFile(certificate.Rendition+"/index.m3u8") || certificate.Timeline != sha256.Sum256(timeline) {
		return certificate, errCopiedHLSIndex
	}
	return certificate, nil
}

func (manager *hlsManager) copiedHLSClockAdmission(ctx context.Context) (context.Context, func(), error) {
	if ctx.Value(copiedHLSMetadataKey{}) == manager {
		return ctx, func() {}, nil
	}
	return manager.copiedHLSMetadataAdmission(ctx)
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
		return context.WithValue(ctx, copiedHLSMetadataKey{}, manager), func() { cancel(); <-gate }, nil
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
	manager.cacheCopiedHLSEndpoint(directory, policy, manifest, assets, end)
	return end, nil
}

func (manager *hlsManager) cacheCopiedHLSEndpoint(directory, policy string, manifest []byte, assets []os.FileInfo, end float64) {
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	if len(metadata.endpoints) >= 64 {
		clear(metadata.endpoints)
	}
	metadata.endpoints[directory] = copiedHLSEndpoint{policy: policy, manifest: sha256.Sum256(manifest), assets: assets, end: end}
	metadata.mu.Unlock()
}
