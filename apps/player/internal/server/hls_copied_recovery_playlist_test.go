package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopiedRecoveryIndexedMismatchCannotFallBackToPhysicalEOF(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	projection := manager.copiedPlaylistProjection(t.Context(), item, recipe, directory, "360p", policy)
	first := projection([]byte(copiedRecoveryManifest))
	if len(first) == 0 || !bytes.Contains(first, []byte("#EXT-X-PLAYLIST-TYPE:VOD")) {
		t.Fatal("valid indexed projection missing")
	}
	bad := []byte(strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,", "#EXTINF:2.400000,", 1))
	if got := projection(bad); len(got) != 0 {
		t.Fatal("indexed mismatch fell back to uncertified physical EOF")
	}
	if got := projection([]byte(copiedRecoveryManifest)); !bytes.Equal(got, first) {
		t.Fatal("published VOD metadata changed")
	}
}

func TestCopiedRecoveryIndexedReuseChecksManifestAndInit(t *testing.T) {
	for _, damage := range []string{"manifest", "init", "first", "same-stat-init", "same-stat-first", "map", "source"} {
		t.Run(damage, func(t *testing.T) {
			manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
			copiedRecoveryProbe(t, manager, "")
			if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
				t.Fatal(err)
			}
			if !manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
				t.Fatal("valid indexed cache rejected")
			}
			switch damage {
			case "same-stat-init", "same-stat-first":
				name := "init.mp4"
				if damage == "same-stat-first" {
					name = "segment-00000.m4s"
				}
				path := filepath.Join(directory, "360p", name)
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				writeHLSLoadingFile(t, path, strings.Repeat("x", int(info.Size())))
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "init":
				writeHLSLoadingFile(t, filepath.Join(directory, "360p/init.mp4"), "replacement init")
			case "first":
				writeHLSLoadingFile(t, filepath.Join(directory, "360p/segment-00000.m4s"), "replacement first")
			case "source":
				writeHLSLoadingFile(t, filepath.Join(directory, ".source"), "different policy")
			case "map":
				timeline.End = 4.05
				data, err := json.Marshal(timeline)
				if err != nil {
					t.Fatal(err)
				}
				writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), string(data))
			default:
				writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), strings.Replace(copiedRecoveryManifest, "2.000000", "2.400000", 1))
			}
			if manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
				t.Fatal("damaged indexed cache remained reusable")
			}
		})
	}
}

func TestCopiedRecoveryPendingClockCannotPublishRawEOF(t *testing.T) {
	manager, item, recipe, directory, policy, _ := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	projections := []func([]byte) []byte{manager.copiedPlaylistProjection(t.Context(), item, recipe, directory, "360p", policy), manager.copiedStartupProjection(item, recipe, directory)}
	for _, projection := range projections {
		if got := projection([]byte(copiedRecoveryManifest)); len(got) != 0 {
			t.Fatal("pending indexed clock exposed raw EOF before fixed VOD commitment")
		}
	}
}

func TestCopiedRecoveryMissingLazyFragmentAndAbsentIndexRemainReusable(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"360p/segment-00001.m4s", "360p/segment-00000.m4s", ".copy-timeline"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
		if !manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
			t.Fatalf("missing %s discarded reusable cache", name)
		}
	}
}

func TestCopiedRecoveryEOFCorrectionCannotExceedCommittedTarget(t *testing.T) {
	manifest := []byte(strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,\nsegment-00001.m4s", "#EXTINF:2.450000,\nsegment-00001.m4s", 1))
	if got := completedCopiedHLSManifest(manifest, 4.55); len(got) != 0 {
		t.Fatal("late 2.45-to-2.55 correction was admitted under target2")
	}
}

func TestCopiedRecoveryEOFCorrectionValidatesSerializedTarget(t *testing.T) {
	manifest := strings.Replace(copiedRecoveryManifest, "#EXTINF:2.000000,\nsegment-00001.m4s", "#EXTINF:2.450000,\nsegment-00001.m4s", 1)
	if got := completedCopiedHLSManifest([]byte(manifest), 4.4999996); len(got) != 0 {
		t.Fatal("six-decimal 2.500000 cut exceeded committed target2")
	}
	if got := completedCopiedHLSManifest([]byte(manifest), 4.4999994); len(got) == 0 {
		t.Fatal("safe serialized 2.499999 cut rejected")
	}
	for _, target := range []string{"NaN", "+Inf", "2.1", "2.0", "2e0", "+2", " 2", "2\n#EXT-X-TARGETDURATION:2"} {
		bad := strings.Replace(copiedRecoveryManifest, "#EXT-X-TARGETDURATION:2", "#EXT-X-TARGETDURATION:"+target, 1)
		if got := completedCopiedHLSManifest([]byte(bad), 4.05); len(got) != 0 {
			t.Fatalf("unsafe target %q accepted", target)
		}
	}
}

func TestCopiedRecoveryIndexedReadRequiresBoundPhysicalManifest(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), strings.Replace(copiedRecoveryManifest, "2.000000", "2.400000", 1))
	if _, err := manager.readCopiedHLSTimelineContext(t.Context(), directory, policy); err == nil {
		t.Fatal("indexed read admitted a mismatched physical manifest before seek/reuse")
	}
}
