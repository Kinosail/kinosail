package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Existing controlled metadata fixtures isolate cache trust/cancellation;
// only hosted real source/decode tests can certify actual key/frame identity.
func TestSeekPlanKeepsCertifiedKeyAndRejectsInvalidCertificate(t *testing.T) {
	manager, item, recipe, old, _, timeline := copiedRecoveryFixture(t)
	recipe.offset = 2
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	if err := os.Rename(old, directory); err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\n360p/index.m3u8\n")
	timeline.Policy, timeline.Keys, timeline.End = options.Cache, []copiedHLSKey{{PTS: 2000, DTS: 2000}, {PTS: 4000, DTS: 4000}}, 6
	if err := manager.writeCopiedHLSTimeline(directory, timeline); err != nil {
		t.Fatal(err)
	}
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", options.Cache, timeline); err != nil {
		t.Fatal(err)
	}
	if ok, err := manager.certifiedCopiedSeek(t.Context(), item, recipe); err != nil || !ok {
		t.Fatalf("certified key rejected: %v", err)
	}
	facts := mediaFactsFor(item, manager.probe.facts(t.Context(), item))
	client := browserPlaybackCapabilities(manager.settings, nil)
	allowed := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	plan := playbackWithAutomaticSkip(facts, client, allowed, NetworkIntent{PreferCompatibility: true}, nil, nil)
	selected, err := manager.exactSeekPlan(t.Context(), item, facts, client, ViewerPolicy{AllowPlayback: true}, plan, recipe, 2)
	if err != nil || selected.Allowed || selected.Mode != "denied" {
		t.Fatal("certification bypassed current conversion permission")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if ok, err := manager.certifiedCopiedSeek(ctx, item, recipe); err == nil || ok {
		t.Fatal("cancellation became copy admission")
	}
	writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), `{}`)
	if ok, err := manager.certifiedCopiedSeek(t.Context(), item, recipe); err == nil || ok {
		t.Fatal("invalid present certificate became absent/fallback admission")
	}
}

func TestSeekPlanAbsentCertificateDoesNotProbeOrCreateCache(t *testing.T) {
	manager, item, recipe, old, _, _ := copiedRecoveryFixture(t)
	arguments := filepath.Join(filepath.Dir(item.Path), "probe-arguments")
	before, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	recipe.offset = 12.5
	if ok, err := manager.certifiedCopiedSeek(t.Context(), item, recipe); err != nil || ok {
		t.Fatalf("missing certificate=%v %v", ok, err)
	}
	if _, err := os.Stat(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))); !os.IsNotExist(err) {
		t.Fatal("planning created an uncertified generation")
	}
	if len(manager.jobs) != 0 {
		t.Fatal("planning started producer")
	}
	after, err := os.ReadFile(arguments)
	if err != nil || string(before) != string(after) {
		t.Fatal("cold planning started a source/packet probe")
	}
}

func TestSeekCertificateRejectsUnsafePresentGenerationAndTimeline(t *testing.T) {
	for _, unsafe := range []string{"generation-symlink", "timeline-symlink", "timeline-hardlink"} {
		t.Run(unsafe, func(t *testing.T) {
			manager, item, recipe, directory, _, _ := copiedRecoveryFixture(t)
			timeline := filepath.Join(directory, ".copy-timeline")
			if unsafe == "generation-symlink" {
				retired := directory + "-retired"
				if err := os.Rename(directory, retired); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(retired, directory); err != nil {
					t.Fatal(err)
				}
			} else {
				retired := filepath.Join(t.TempDir(), "timeline")
				if err := os.Rename(timeline, retired); err != nil {
					t.Fatal(err)
				}
				var err error
				if unsafe == "timeline-symlink" {
					err = os.Symlink(retired, timeline)
				} else {
					err = os.Link(retired, timeline)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if ok, err := manager.certifiedCopiedSeek(t.Context(), item, recipe); err == nil || ok {
				t.Fatal("unsafe certificate acquired copy or fallback admission")
			}
			if len(manager.jobs) != 0 {
				t.Fatal("rejected certificate started a producer")
			}
		})
	}
}

func TestSeekPlanPreservesSelectionAndConversionPolicy(t *testing.T) {
	manager, item, _, old, _, _ := copiedRecoveryFixture(t)
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	facts := mediaFactsFor(item, manager.probe.facts(t.Context(), item))
	facts.Audio = append(facts.Audio, facts.Audio[0])
	facts.Audio[1].Index, facts.Audio[1].SourceIndex = 1, 2
	client := browserPlaybackCapabilities(manager.settings, nil)
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	audio := 1
	intent := NetworkIntent{PreferCompatibility: true, AudioIndex: &audio, MaxBitrate: 1_000_000}
	plan := playbackWithAutomaticSkip(facts, client, policy, intent, nil, nil)
	recipe := recipeFor(plan)
	actual, err := manager.exactSeekPlan(t.Context(), item, facts, client, policy, plan, recipe, 12.5)
	if err != nil || actual.Mode != "transcode" || actual.AudioIndex != 1 || actual.MaxBitrate != plan.MaxBitrate {
		t.Fatalf("selection lost: %+v %v", actual, err)
	}
	policy.AllowTranscode = false
	actual, err = manager.exactSeekPlan(t.Context(), item, facts, client, policy, plan, recipe, 12.5)
	if err != nil || actual.Allowed || actual.Mode != "denied" {
		t.Fatal("seek bypassed conversion policy")
	}
	for _, position := range []float64{-1, 12.55, facts.Duration, 604801} {
		if _, err := manager.exactSeekPlan(t.Context(), item, facts, client, policy, plan, recipe, position); err == nil {
			t.Fatal("invalid seek admitted")
		}
	}
	if len(manager.jobs) != 0 {
		t.Fatal("rejected/denied plan started producer")
	}
}
