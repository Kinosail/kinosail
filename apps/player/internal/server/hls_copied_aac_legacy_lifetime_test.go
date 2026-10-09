package server

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type copiedAACLegacyEntry struct {
	info os.FileInfo
	hash [32]byte
}

// Gap: the old limited content map cannot detect new entries or equal-byte writes.
func copiedAACLegacyInventory(t *testing.T, directory string) map[string]copiedAACLegacyEntry {
	t.Helper()
	result := map[string]copiedAACLegacyEntry{}
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		if len(result) >= 128 || !info.IsDir() && (!info.Mode().IsRegular() || info.Size() > 2<<20) {
			t.Fatal("legacy test inventory exceeded its bounded fixture")
		}
		value := copiedAACLegacyEntry{info: info}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.hash = sha256.Sum256(data)
		}
		result[name] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
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
		})
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
