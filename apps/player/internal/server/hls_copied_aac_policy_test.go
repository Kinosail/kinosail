package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// Gap: public media cannot schedule equal-stat inode replacement or table capacity.
func TestCopiedAACPolicyKeepsPositiveUntilSourceChanges(t *testing.T) {
	manager, item, recipe, _, _, _ := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager, item)
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	key := hlsRecipeKey(item.ID, recipe)
	base := "source-policy"
	if selected, _, known := manager.copiedAACPolicy(key, base, info); known || selected {
		t.Fatal("absent qualification became a completed unsupported result")
	}
	if err := manager.recordCopiedAACPolicy(key, base, info, true, 1); err != nil {
		t.Fatal(err)
	}
	if err := manager.recordCopiedAACPolicy(key, base, info, false, 0); err == nil {
		t.Fatal("completed unsupported decision erased a positive for the same source")
	}
	copiedAACPolicyCapacity(t, manager, key, base, info)
	if _, _, known := manager.copiedAACPolicy(key, base+"-changed", info); known {
		t.Fatal("changed base policy inherited an older qualification")
	}
	changed := copiedAACEqualStatReplacement(t, item.Path, info)
	if _, _, known := manager.copiedAACPolicy(key, base, changed); known {
		t.Fatal("replacement inode inherited a source qualification")
	}
}

func TestCopiedAACPolicyCannotExposeLegacyIndexedAssets(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, item, recipe, directory, policy, before, info := copiedAACLegacyIndexedFixture(t)
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), policy, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil || !strings.HasSuffix(options.Cache, ":copied-aac=2") {
		t.Fatal("qualified source did not acquire an isolated producer policy")
	}
	if manager.startupWindowReady(manager.ctx, item, recipe) || manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
		t.Fatal("qualified source returned Version1 readiness or reuse")
	}
	for _, name := range []string{"360p/init.mp4", "360p/segment-00000.m4s"} {
		if err := manager.copiedAACCacheAsset(t.Context(), item, recipe, directory, name); err == nil {
			t.Fatalf("existing %s bypassed the required producer certificate", name)
		}
	}
	copiedAACPreservedTimeline(t, directory, before)
}

func TestCopiedAACReadinessUnconfiguredRetainsFalse(t *testing.T) {
	manager := &hlsManager{}
	if manager.startupWindowReady(t.Context(), library.Item{Path: "fixture.mp4"}, hlsRecipe{mode: "remux"}) {
		t.Fatal("unconfigured readiness became ready")
	}
}

func copiedAACSourceRoots(manager *hlsManager, item library.Item) {
	manager.index = memoryLibraryIndex([]library.Item{item}, true)
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
}

func copiedAACPolicyCapacity(t *testing.T, manager *hlsManager, key, base string, info os.FileInfo) {
	t.Helper()
	for n := 1; n < maximumCopiedAACPolicies; n++ {
		if err := manager.recordCopiedAACPolicy("other-"+strconv.Itoa(n), base, info, false, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.recordCopiedAACPolicy("overflow", base, info, false, 0); err == nil {
		t.Fatal("qualification capacity was not bounded")
	}
	for _, sample := range []struct {
		key             string
		selected, known bool
		track           int
	}{{"other-1", false, true, 0}, {"overflow", false, false, 0}, {key, true, true, 1}} {
		selected, track, known := manager.copiedAACPolicy(sample.key, base, info)
		if selected != sample.selected || known != sample.known || sample.selected && track != sample.track {
			t.Fatal("capacity changed completed unsupported, rejected overflow or admitted source decisions")
		}
	}
}

func copiedAACEqualStatReplacement(t *testing.T, source string, before os.FileInfo) os.FileInfo {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(source), "replacement")
	writeHLSLoadingFile(t, replacement, string(data))
	if err := os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, source); err != nil {
		t.Fatal(err)
	}
	changed, err := os.Stat(source)
	if err != nil || changed.Size() != before.Size() || !changed.ModTime().Equal(before.ModTime()) || os.SameFile(before, changed) {
		t.Fatal("equal-stat replacement control failed")
	}
	return changed
}

func copiedAACLegacyIndexedFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string, string, []byte, os.FileInfo) {
	t.Helper()
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager, item)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	if !manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
		t.Fatal("legacy control was not reusable before qualification")
	}
	before, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	return manager, item, recipe, directory, policy, before, info
}

func copiedAACPreservedTimeline(t *testing.T, directory string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
	if err != nil || string(after) != string(before) {
		t.Fatal("readiness rejection changed preserved Version1 metadata")
	}
}
