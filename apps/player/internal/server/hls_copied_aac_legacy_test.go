package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// Gap: public HTTP cannot revoke a source/root or replace one exact opened
// generation between admission and rendering. This is metadata-only evidence.
func copiedAACLegacyFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string, string) {
	t.Helper()
	manager, item, recipe, directory, base, timeline := copiedRecoveryFixture(t)
	original := item.Path
	item.Path = filepath.Join(filepath.Dir(original), "Fixture.mp4")
	if err := os.Rename(original, item.Path); err != nil {
		t.Fatal(err)
	}
	copiedAACSourceRoots(manager, item)
	clock := 0.083333
	timeline.Clock = &clock
	data, err := json.Marshal(timeline)
	if err != nil {
		t.Fatal(err)
	}
	initialization, err := os.ReadFile(filepath.Join(directory, "360p/init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(directory, "360p/segment-00000.m4s"))
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := json.Marshal(copiedHLSClockCertificate{Version: 1, Rendition: "360p",
		Timeline: sha256.Sum256(data), Initialization: sha256.Sum256(initialization), First: sha256.Sum256(first)})
	if err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), string(data))
	writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), string(certificate))
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), base, info, true, 1); err != nil {
		t.Fatal(err)
	}
	return manager, item, recipe, directory, base
}

func TestCopiedAACLegacyReadDoesNotErasePositiveEligibility(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	manager, item, recipe, directory, base := copiedAACLegacyFixture(t)
	before := copiedAACLegacyInventory(t, directory)
	held, err := manager.openCopiedHLSLegacyGeneration(t.Context(), item, recipe, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer held.close()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)
	if _, err := held.playlist("index.m3u8", 0, 4, request); err != nil {
		t.Fatal("valid legacy read was rejected while Version2 eligibility remained positive")
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	selected, track, known := manager.copiedAACPolicy(hlsRecipeKey(item.ID, recipe), base, info)
	if !known || !selected || track != 1 {
		t.Fatal("legacy read changed the positive producer decision")
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil || !copiedAACPolicyRequired(options.Cache) {
		t.Fatal("legacy read changed effective Version2 producer settings")
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACLegacyInventory(t, directory))
}

func TestCopiedAACLegacyRetainedReadRejectsSourceRootAndGenerationChanges(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	for _, damage := range []string{"source", "source-root", "generation", "rendition", "source-binding", "master", "manifest", "certificate", "timeline"} {
		t.Run(damage, func(t *testing.T) {
			manager, item, recipe, directory, _ := copiedAACLegacyFixture(t)
			held, err := manager.openCopiedHLSLegacyGeneration(t.Context(), item, recipe, directory)
			if err != nil {
				t.Fatal(err)
			}
			defer held.close()
			damageCopiedAACLegacy(t, manager, item, directory, damage)
			if held.current() {
				t.Fatal("changed source, policy or generation retained legacy admission")
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)
			if _, err := held.playlist("index.m3u8", 0, 4, request); err == nil {
				t.Fatal("changed legacy generation reached playlist rendering")
			}
		})
	}
}

func TestCopiedAACLegacySlowNetworkReleasesLeaseAndRetainsOpenedAsset(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("selected compatibility reader is Linux-only")
	}
	manager, item, recipe, directory, _ := copiedAACLegacyFixture(t)
	owner := newCopiedAACDelivery(t)
	request := httptest.NewRequestWithContext(owner.ctx, http.MethodGet, "/segment-00001.m4s", nil)
	go func() {
		owner.done <- manager.serveCopiedHLSLegacy(owner.writer, request, item, recipe, "360p/segment-00001.m4s")
	}()
	owner.waitEntered(t, "legacy media did not reach retained delivery")
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		t.Fatal("slow legacy media retained metadata admission")
	}
	release()
	path := filepath.Join(directory, "360p/segment-00001.m4s")
	if err := os.Rename(path, path+"-retired"); err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, path, "replacement bytes")
	owner.release()
	owner.join(t, "retained legacy delivery failed")
	if owner.writer.Body.String() != "last fragment" {
		t.Fatal("network delivery reopened a replacement canonical path")
	}
}

func damageCopiedAACLegacy(t *testing.T, manager *hlsManager, item library.Item, directory, damage string) {
	t.Helper()
	switch damage {
	case "source":
		info, err := os.Stat(item.Path)
		if err != nil {
			t.Fatal(err)
		}
		copiedAACEqualStatReplacement(t, item.Path, info)
	case "source-root":
		manager.index.SetRoots(nil)
	case "generation", "rendition":
		name := map[string]string{"generation": directory, "rendition": filepath.Join(directory, "360p")}[damage]
		replaceCopiedAACLegacyDirectory(t, name)
	case "certificate", "timeline":
		name := map[string]string{"certificate": ".copy-clock", "timeline": ".copy-timeline"}[damage]
		replaceCopiedAACLegacyMetadata(t, filepath.Join(directory, name))
	case "source-binding":
		writeHLSLoadingFile(t, filepath.Join(directory, ".source"), "wrong policy")
	case "master":
		writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n360p/index.m3u8\n")
	case "manifest":
		writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"),
			strings.Replace(copiedRecoveryManifest, "2.000000", "2.400000", 1))
	}
}

func replaceCopiedAACLegacyDirectory(t *testing.T, name string) {
	t.Helper()
	retired := name + "-retired"
	if err := os.Rename(name, retired); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(name, os.DirFS(retired)); err != nil {
		t.Fatal(err)
	}
}

func replaceCopiedAACLegacyMetadata(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-retired"); err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, path, string(data))
}

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
