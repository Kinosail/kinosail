package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopiedRecoveryRefillRejectsUncertifiedOutput(t *testing.T) {
	for _, sample := range []struct{ init, first, manifest string }{
		{"damaged init", "first fragment", copiedRecoveryManifest},
		{"initialization", "damaged first", copiedRecoveryManifest},
		{"initialization", "first fragment", strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,", "#EXTINF:1.000000,", 1)},
	} {
		t.Run(sample.init+"/"+sample.first+"/"+sample.manifest[0:10], func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			check := copiedRecoveryPreserved(t, directory)
			copiedRecoveryEncoderOutput(t, manager, "", sample.init, sample.first, sample.manifest)
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err == nil {
				t.Error("uncertified output acquired the committed cache")
			}
			check()
			if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
				t.Fatal("uncertified media was published")
			}
		})
	}
}

func TestCopiedRecoveryRefillRejectsStageResourceAndFileViolations(t *testing.T) {
	for _, action := range []string{
		"i=0; while [ \"$i\" -lt 9 ]; do printf x > \"$directory/unexpected-$i\"; i=$((i+1)); done",
		"trap 'rm \"$directory/segment-00000.m4s\"; ln -s init.mp4 \"$directory/segment-00000.m4s\"' EXIT",
		"trap 'python3 -c '\"'\"'import os,sys; os.truncate(sys.argv[1],67108865)'\"'\"' \"$directory/segment-00000.m4s\"' EXIT",
	} {
		t.Run(action[0:5], func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			check := copiedRecoveryPreserved(t, directory)
			copiedRecoveryEncoder(t, manager, action)
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err == nil {
				t.Error("invalid staged file set or resource limit was accepted")
			}
			check()
			if _, err := os.Lstat(filepath.Join(directory, "360p/segment-00000.m4s")); !os.IsNotExist(err) {
				t.Fatal("invalid staged output reached the canonical rendition")
			}
		})
	}
}

func TestCopiedRecoveryRefillRejectsBadIndexBeforeCodecEffects(t *testing.T) {
	for _, damage := range []string{"malformed", "nonregular-first"} {
		t.Run(damage, func(t *testing.T) {
			manager, item, recipe, directory := copiedRecoveryRefill(t)
			if damage == "malformed" {
				writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), "malformed")
			} else if err := os.Symlink("init.mp4", filepath.Join(directory, "360p/segment-00000.m4s")); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "codec-started")
			copiedRecoveryEncoder(t, manager, "printf started > "+copiedRecoveryQuote(marker))
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err == nil {
				t.Error("bad existing indexed cache fell into the legacy writer")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("invalid existing index started a codec")
			}
		})
	}
}

func TestCopiedRecoveryRefillSingleGOPUsesGenuineEOF(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	timeline.Keys, timeline.End = timeline.Keys[:1], 2
	manifest := strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,\nsegment-00001.m4s\n", "", 1)
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), manifest)
	if err := manager.writeCopiedHLSTimeline(directory, timeline); err != nil {
		t.Fatal(err)
	}
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "360p/segment-00000.m4s")); err != nil {
		t.Fatal(err)
	}
	check := copiedRecoveryPreserved(t, directory)
	copiedRecoveryEncoderOutput(t, manager, "", "initialization", "first fragment", manifest)
	if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err != nil {
		t.Fatal("single-GOP EOF refill was rejected")
	}
	check()
}

func TestCopiedRecoveryPendingAndUnindexedColdOutputRemainAvailable(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "pending"}[indexed], func(t *testing.T) {
			manager, item, recipe, directory, _, _ := copiedRecoveryFixture(t)
			if !indexed {
				if err := os.Remove(filepath.Join(directory, ".copy-timeline")); err != nil {
					t.Fatal(err)
				}
			}
			copiedRecoveryEncoder(t, manager, "")
			if err := copiedRecoveryRunRefill(t.Context(), manager, item, recipe, directory); err != nil {
				t.Fatal("ordinary cold encoder admission changed")
			}
		})
	}
}
