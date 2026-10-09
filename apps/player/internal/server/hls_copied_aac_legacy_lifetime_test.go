package server

import (
	"context"
	"crypto/sha256"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

type copiedAACLegacyEntry struct {
	info os.FileInfo
	hash [32]byte
}

// Gap: the old limited content map cannot detect new entries or equal-byte writes.
func copiedAACLegacyInventory(t *testing.T, directory string) map[string]copiedAACLegacyEntry {
	t.Helper()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	result := map[string]copiedAACLegacyEntry{}
	err = fs.WalkDir(root.FS(), ".", func(name string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(result) >= 128 {
			t.Fatal("legacy test inventory exceeded its bounded fixture")
		}
		value, err := copiedAACLegacyInventoryEntry(root, name)
		if err != nil {
			return err
		}
		result[name] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func copiedAACLegacyInventoryEntry(root *os.Root, name string) (copiedAACLegacyEntry, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return copiedAACLegacyEntry{}, err
	}
	value := copiedAACLegacyEntry{info: info}
	if info.IsDir() {
		return value, nil
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return value, errCopiedHLSIndex
	}
	data, err := copiedAACLegacyInventoryContent(root, name, info)
	value.hash = sha256.Sum256(data)
	return value, err
}

func copiedAACLegacyInventoryContent(root *os.Root, name string, before os.FileInfo) ([]byte, error) {
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameCopiedHLSFile(before, opened) {
		return nil, errCopiedHLSIndex
	}
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	current, statErr := root.Lstat(name)
	if err != nil || statErr != nil || int64(len(data)) != before.Size() || !sameCopiedHLSFile(before, current) {
		return nil, errCopiedHLSIndex
	}
	return data, nil
}

func requireCopiedAACLegacyInventory(t *testing.T, before, after map[string]copiedAACLegacyEntry) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatal("legacy read added or removed a cache entry")
	}
	for name, previous := range before {
		current, ok := after[name]
		if !ok || previous.hash != current.hash || !os.SameFile(previous.info, current.info) ||
			previous.info.Mode() != current.info.Mode() || previous.info.Size() != current.info.Size() ||
			!previous.info.ModTime().Equal(current.info.ModTime()) {
			t.Fatal("legacy read changed cache content or identity")
		}
	}
}

func TestCopiedAACLegacyCancellationAndCloseReleaseDescriptors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	manager, item, recipe, directory, _ := copiedAACLegacyFixture(t)
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancelBefore := context.WithCancel(t.Context())
	cancelBefore()
	if held, err := manager.openCopiedHLSLegacyGeneration(canceled, item, recipe, directory); err == nil {
		held.close()
		t.Fatal("canceled caller acquired a legacy generation")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held, err := manager.openCopiedHLSLegacyGeneration(ctx, item, recipe, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer held.close()
	cancel()
	if held.current() {
		t.Fatal("post-open cancellation retained legacy admission")
	}
	held.close()
	held.close()
	_, release, err := manager.copiedHLSMetadataAdmission(t.Context())
	if err != nil {
		t.Fatal("closed legacy generation retained metadata admission")
	}
	release()
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil || len(before) != len(after) {
		t.Fatal("legacy close did not restore the descriptor baseline")
	}
}

func TestCopiedAACLegacyMissingTimelineIsHandledBeforePreparation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	for _, warm := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold", true: "sticky-positive"}[warm], func(t *testing.T) {
			requireCopiedAACLegacyMissingTimeline(t, warm)
		})
	}
}

func requireCopiedAACLegacyMissingTimeline(t *testing.T, warm bool) {
	t.Helper()
	manager, item, recipe, directory, base := copiedAACLegacyFixture(t)
	key := hlsRecipeKey(item.ID, recipe)
	if !warm {
		manager.copiedMetadata.mu.Lock()
		delete(manager.copiedMetadata.aacPolicies, key)
		manager.copiedMetadata.mu.Unlock()
	}
	if err := os.Remove(filepath.Join(directory, ".copy-timeline")); err != nil {
		t.Fatal(err)
	}
	before := copiedAACLegacyInventory(t, directory)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)
	writer := httptest.NewRecorder()
	if !manager.serveCopiedHLSLegacy(writer, request, item, recipe, "index.m3u8") || writer.Code != http.StatusNotFound {
		t.Fatal("missing indexed timeline fell through to preparation")
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACLegacyInventory(t, directory))
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	selected, _, known := manager.copiedAACPolicy(key, base, info)
	if selected != warm || known != warm {
		t.Fatal("rejection changed the producer decision")
	}
}

func TestCopiedAACLegacyClosedInheritedLeaseRejectsSafely(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	manager, item, recipe, directory, _ := copiedAACLegacyFixture(t)
	ctx, release, err := manager.copiedHLSClockAdmission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	held, err := manager.openCopiedHLSLegacyGeneration(ctx, item, recipe, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer held.close()
	held.close()
	if held.current() {
		t.Fatal("closed generation remained admitted under a caller-owned lease")
	}
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/index.m3u8", nil)
	if _, err := held.playlist("index.m3u8", 0, 4, request); err == nil {
		t.Fatal("closed generation reached playlist rendering")
	}
}
