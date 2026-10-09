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
	for n := 1; n < maximumCopiedAACPolicies; n++ {
		if err := manager.recordCopiedAACPolicy("other-"+strconv.Itoa(n), base, info, false, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.recordCopiedAACPolicy("overflow", base, info, false, 0); err == nil {
		t.Fatal("qualification capacity was not bounded")
	}
	if selected, _, known := manager.copiedAACPolicy("other-1", base, info); !known || selected {
		t.Fatal("completed unsupported qualification remained unknown")
	}
	if selected, _, known := manager.copiedAACPolicy("overflow", base, info); known || selected {
		t.Fatal("capacity failure acquired an unsupported or positive result")
	}
	selected, track, known := manager.copiedAACPolicy(key, base, info)
	if !known || !selected || track != 1 {
		t.Fatal("capacity erased an admitted source instead of rejecting new work")
	}
	if _, _, known := manager.copiedAACPolicy(key, base+"-changed", info); known {
		t.Fatal("changed base policy inherited an older qualification")
	}
	data, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(item.Path), "replacement")
	writeHLSLoadingFile(t, replacement, string(data))
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, item.Path); err != nil {
		t.Fatal(err)
	}
	changed, err := os.Stat(item.Path)
	if err != nil || changed.Size() != info.Size() || !changed.ModTime().Equal(info.ModTime()) || os.SameFile(info, changed) {
		t.Fatal("equal-stat replacement control failed")
	}
	if _, _, known := manager.copiedAACPolicy(key, base, changed); known {
		t.Fatal("replacement inode inherited a source qualification")
	}
}

func TestCopiedAACPolicyCannotExposeLegacyIndexedAssets(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
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
	if err := manager.recordCopiedAACPolicy(hlsRecipeKey(item.ID, recipe), policy, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil || !strings.HasSuffix(options.Cache, ":copied-aac=2") {
		t.Fatal("qualified source did not acquire an isolated producer policy")
	}
	if manager.startupWindowReady(item, recipe) || manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
		t.Fatal("qualified source returned Version1 readiness or reuse")
	}
	for _, name := range []string{"360p/init.mp4", "360p/segment-00000.m4s"} {
		if err := manager.copiedAACCacheAsset(t.Context(), item, recipe, directory, name); err == nil {
			t.Fatalf("existing %s bypassed the required producer certificate", name)
		}
	}
	after, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
	if err != nil || string(after) != string(before) {
		t.Fatal("readiness rejection changed preserved Version1 metadata")
	}
}

func TestCopiedAACReadinessUnconfiguredRetainsFalse(t *testing.T) {
	manager := &hlsManager{}
	if manager.startupWindowReady(library.Item{Path: "fixture.mp4"}, hlsRecipe{mode: "remux"}) {
		t.Fatal("unconfigured readiness became ready")
	}
}

func copiedAACSourceRoots(manager *hlsManager, item library.Item) {
	manager.index = memoryLibraryIndex([]library.Item{item}, true)
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
}
