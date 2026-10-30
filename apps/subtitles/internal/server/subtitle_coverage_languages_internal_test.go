package server

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func coverageManager(languages ...string) *subtitleManager {
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: languages[0], SubtitleLanguages: languages}, persist: func(string, any) error { return nil }}
	return &subtitleManager{settings: settings}
}

func TestUntaggedSidecarCountsOnlyForThePrimaryLanguage(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(directory, "Film.mkv"), Subtitles: []string{filepath.Join(directory, "Film.srt")}}
	manager := coverageManager("en", "fr")

	ready, _, missing := manager.planCoverageAllFacts(item, []string{"en", "fr"}, probeResult{})
	if ready || !slices.Equal(missing, []string{"fr"}) {
		t.Fatalf("plan ready=%t missing=%v", ready, missing)
	}
	if french, _ := manager.sidecarPlanCoverage(item, "fr"); french {
		t.Fatal("an untagged sidecar satisfied the second preferred language")
	}
	if english, _ := manager.sidecarPlanCoverage(item, "en"); !english {
		t.Fatal("an untagged sidecar did not satisfy the primary language")
	}
	_, _, manual := subtitleCoverageAll(item, []string{"en", "fr"})
	if !slices.Equal(manual, missing) {
		t.Fatalf("automation missing %v, manual missing %v", missing, manual)
	}
}

func TestSidecarNamedWithAFullLanguageNameCountsForThatLanguage(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for _, name := range []string{"Film.English.srt", "Film.english.forced.srt", "Film.Portuguese.srt"} {
		item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(directory, "Film.mkv"), Subtitles: []string{filepath.Join(directory, name)}}
		_, labels, missing := subtitleCoverageAll(item, []string{"en", "pt"})
		switch name {
		case "Film.English.srt":
			if !slices.Equal(missing, []string{"pt"}) || !slices.Equal(labels, []string{"en"}) {
				t.Fatalf("%s: labels=%v missing=%v", name, labels, missing)
			}
		case "Film.english.forced.srt":
			if !slices.Equal(missing, []string{"en", "pt"}) {
				t.Fatalf("%s: forced track counted as full coverage: %v", name, missing)
			}
		default:
			if !slices.Equal(missing, []string{"en"}) {
				t.Fatalf("%s: labels=%v missing=%v", name, labels, missing)
			}
		}
	}
	if language := subtitleLanguageFromPath(filepath.Join(directory, "Film.mkv"), filepath.Join(directory, "Film.Spanish.srt")); language != "es" {
		t.Fatalf("language from full name = %q", language)
	}
}
