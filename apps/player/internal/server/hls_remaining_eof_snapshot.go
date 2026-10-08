package server

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type remainingColdAACState struct {
	manifest []byte
	files    map[string]fs.FileInfo
}

func (manager *hlsManager) remainingColdAACSnapshot(ctx context.Context, item library.Item, recipe hlsRecipe, key, policy string, job *hlsJob, duration float64, generation ...*remainingColdAACState) (*remainingColdAACState, error) {
	if err := manager.remainingColdAACStable(ctx, item, recipe, key, policy, job); err != nil {
		return nil, err
	}
	root, err := manager.openCopiedHLSRoot(filepath.Join(manager.cache, key))
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	state, err := remainingColdAACAssets(root, generation...)
	if err != nil {
		return nil, err
	}
	if err := manager.remainingColdAACVerifyGeneration(key, state); err != nil {
		return nil, err
	}
	if err := state.readComplete(ctx, root, policy, duration); err != nil {
		return nil, err
	}
	if err := state.verifyFiles(root); err != nil {
		return nil, err
	}
	if err := manager.remainingColdAACStable(ctx, item, recipe, key, policy, job); err != nil {
		return nil, err
	}
	if err := manager.remainingColdAACVerifyGeneration(key, state); err != nil {
		return nil, err
	}
	return state, nil
}

func remainingColdAACAssets(root *os.Root, generation ...*remainingColdAACState) (*remainingColdAACState, error) {
	if _, err := root.Lstat(".copy-timeline"); !os.IsNotExist(err) {
		return nil, errCopiedHLSIndex
	}
	state := &remainingColdAACState{files: make(map[string]fs.FileInfo)}
	for _, name := range []string{".", "audio", ".source", "index.m3u8", "audio/index.m3u8", "audio/init.mp4"} {
		if err := state.retain(root, name); err != nil {
			return nil, err
		}
	}
	if len(generation) > 1 || len(generation) == 1 && !state.sameGeneration(generation[0]) {
		return nil, errHLSIdentityChanged
	}
	return state, nil
}

func (state *remainingColdAACState) readComplete(ctx context.Context, root *os.Root, policy string, duration float64) error {
	if err := remainingAACReadyRoot(root, policy); err != nil {
		return err
	}
	manifest, err := remainingAACManifestRead(root)
	if err != nil || !remainingAACManifestValid(manifest) || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return errCopiedHLSIndex
	}
	state.manifest = manifest
	return state.retainSegments(ctx, root, duration)
}

func (state *remainingColdAACState) verifyFiles(root *os.Root) error {
	for name, before := range state.files {
		after, err := root.Lstat(name)
		if err != nil || !remainingColdAACSameFile(before, after) {
			return errHLSIdentityChanged
		}
	}
	return nil
}

func (state *remainingColdAACState) retain(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	directory := name == "." || name == "audio"
	if err != nil || directory && !info.IsDir() || !directory && (!info.Mode().IsRegular() || info.Size() <= 0) {
		return errCopiedHLSIndex
	}
	state.files[name] = info
	return nil
}

func (state *remainingColdAACState) retainSegments(ctx context.Context, root *os.Root, duration float64) error {
	count, total := 0, 0.0
	for _, line := range strings.Split(string(state.manifest), "\n") {
		if err := ctx.Err(); err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		length, err := remainingColdAACLineDuration(line)
		if err != nil {
			return err
		}
		total += length
		if _, valid := hlsSegmentNumber(line); !valid {
			continue
		}
		count++
		if count > 6 {
			return errCopiedHLSIndex
		}
		name := filepath.Join("audio", line)
		if err := remainingAACExistingAsset(root, name); err != nil {
			return err
		}
		if err := state.retain(root, name); err != nil {
			return err
		}
	}
	return remainingColdAACSpan(count, total, duration)
}

func remainingColdAACSameFile(a, b fs.FileInfo) bool {
	if a.IsDir() || b.IsDir() {
		return a.IsDir() && b.IsDir() && os.SameFile(a, b)
	}
	return sameCopiedHLSFile(a, b)
}

func (state *remainingColdAACState) same(other *remainingColdAACState) bool {
	if other == nil || !bytes.Equal(state.manifest, other.manifest) || len(state.files) != len(other.files) {
		return false
	}
	for name, before := range state.files {
		after := other.files[name]
		if after == nil || !remainingColdAACSameFile(before, after) {
			return false
		}
	}
	return true
}

// Capture the recipe and rendition identities before waiting for publication.
func remainingColdAACFragmentDuration(line string) (float64, error) {
	value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	length, err := strconv.ParseFloat(value, 64)
	if err != nil || invalidHLSSegmentDuration(length) {
		return 0, errCopiedHLSIndex
	}
	return length, nil
}

func remainingColdAACSpan(count int, total, duration float64) error {
	if count == 0 || total < duration || total > duration+0.05 {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingColdAACLineDuration(line string) (float64, error) {
	if !strings.HasPrefix(line, "#EXTINF:") {
		return 0, nil
	}
	return remainingColdAACFragmentDuration(line)
}
