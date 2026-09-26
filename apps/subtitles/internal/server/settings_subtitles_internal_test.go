package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleLanguagesValidateAtomicallyAndCanonicalize(t *testing.T) {
	persisted := 0
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { persisted++; return nil }}
	invalid := [][]string{
		nil,
		{"en", "not-a-language"},
		{"pt-br", "pt-BR"},
		{"pt", "pt-BR"},
		{"zh", "zh-Hant"},
		append(make([]string, maximumSubtitleLanguages), "fr"),
	}
	for _, languages := range invalid {
		if err := store.setSubtitleLanguages(languages); err == nil {
			t.Errorf("setSubtitleLanguages(%q) succeeded", languages)
		}
	}
	if persisted != 0 || !slices.Equal(store.subtitleLanguages(), []string{"en"}) {
		t.Fatalf("rejected input changed state: persisted=%d languages=%q", persisted, store.subtitleLanguages())
	}
	if err := store.setSubtitleLanguages([]string{"en", "pt-br", "zh-cn", "es-419"}); err != nil {
		t.Fatal(err)
	}
	if persisted != 1 || !slices.Equal(store.subtitleLanguages(), []string{"en", "pt-BR", "zh-Hans", "es-419"}) {
		t.Fatalf("saved state: persisted=%d languages=%q", persisted, store.subtitleLanguages())
	}
}

func TestSubtitleLanguageWebBoundaryRejectsAmbiguousInputBeforeSave(t *testing.T) {
	persisted := 0
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { persisted++; return nil }}
	handler := saveSubtitleLanguage(store, "/settings#language")
	for _, body := range []string{"language=es&unknown=x", "language=es&language=fr", "language=es&action=add&action=remove", "language=english&action=add", "language=ea", "language=sp", "language=at", "language=pm", "language=zh_bg", "language=es&action="} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%q = %d; want 400", body, response.Code)
		}
	}
	if persisted != 0 || !slices.Equal(store.subtitleLanguages(), []string{"en"}) {
		t.Fatalf("rejected web input changed state: persisted=%d languages=%q", persisted, store.subtitleLanguages())
	}
}

func TestSubtitleLanguageWebFormRejectsConflictingActionVariants(t *testing.T) {
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en"}, persist: func(string, any) error { return nil }}
	for _, test := range []struct {
		name    string
		handler http.Handler
		body    string
	}{
		{name: "primary action", handler: savePrimarySubtitleLanguage(store, "/settings"), body: "language=es&action=add"},
		{name: "action and preference", handler: saveSubtitleLanguage(store, "/settings"), body: "language=es&action=add&preference=sdh"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("%q = %d; want 400", test.body, response.Code)
			}
		})
	}
}

func TestInvalidSubtitleLanguageActionTruthTable(t *testing.T) {
	for _, test := range []struct {
		name                           string
		primary, actionPresent         bool
		action                         string
		preferencePresent, wantInvalid bool
	}{
		{name: "default", wantInvalid: false},
		{name: "preference only", preferencePresent: true, wantInvalid: false},
		{name: "empty action", actionPresent: true, wantInvalid: true},
		{name: "primary action", primary: true, actionPresent: true, action: "add", wantInvalid: true},
		{name: "action and preference", actionPresent: true, action: "add", preferencePresent: true, wantInvalid: true},
		{name: "valid action", actionPresent: true, action: "add", wantInvalid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := invalidSubtitleLanguageAction(test.primary, test.actionPresent, test.action, test.preferencePresent); got != test.wantInvalid {
				t.Fatalf("invalidSubtitleLanguageAction() = %v; want %v", got, test.wantInvalid)
			}
		})
	}
}

func TestSubtitleLanguageWebFormPreservesSecondaryPreferences(t *testing.T) {
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en", SubtitleLanguages: []string{"en", "es", "fr"}}, persist: func(string, any) error { return nil }}
	handler := saveSubtitleLanguage(store, "/settings")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language=fr"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !slices.Equal(store.subtitleLanguages(), []string{"fr", "en", "es"}) {
		t.Fatalf("save = %d languages=%q", response.Code, store.subtitleLanguages())
	}
}

func TestSubtitleLanguageOrderingAndPersistenceFailureHaveNoPartialEffects(t *testing.T) {
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en", SubtitleLanguages: []string{"en", "es", "fr"}}, persist: func(string, any) error { return nil }}
	for _, change := range [][2]string{{"earlier", "fr"}, {"remove", "es"}, {"later", "en"}} {
		if err := store.changeSubtitleLanguages(change[0], change[1]); err != nil {
			t.Fatalf("%s %s: %v", change[0], change[1], err)
		}
	}
	if !slices.Equal(store.subtitleLanguages(), []string{"fr", "en"}) {
		t.Fatalf("ordered languages = %q", store.subtitleLanguages())
	}
	store.persist = func(string, any) error { return errors.New("blocked") }
	if err := store.changeSubtitleLanguages("add", "de"); err == nil {
		t.Fatal("persistence failure was accepted")
	}
	if !slices.Equal(store.subtitleLanguages(), []string{"fr", "en"}) {
		t.Fatalf("persistence failure changed languages: %q", store.subtitleLanguages())
	}
}

func TestSubtitleLanguageAddRejectsOverlappingBaseAndVariant(t *testing.T) {
	persisted := 0
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "pt", SubtitleLanguages: []string{"pt"}}, persist: func(string, any) error { persisted++; return nil }}
	if err := store.changeSubtitleLanguages("add", "pt-BR"); err == nil {
		t.Fatal("overlapping language was accepted")
	}
	if persisted != 0 || !slices.Equal(store.subtitleLanguages(), []string{"pt"}) {
		t.Fatalf("rejected add changed state: persisted=%d languages=%q", persisted, store.subtitleLanguages())
	}
}

func TestPrimarySubtitleLanguagePreservesSecondaryPreferences(t *testing.T) {
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en", SubtitleLanguages: []string{"en", "es", "fr"}}, persist: func(string, any) error { return nil }}
	if err := store.setPrimarySubtitleLanguage("FR"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(store.subtitleLanguages(), []string{"fr", "en", "es"}) {
		t.Fatalf("reordered languages = %q", store.subtitleLanguages())
	}
	if err := store.setPrimarySubtitleLanguage("DE"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(store.subtitleLanguages(), []string{"de", "en", "es"}) {
		t.Fatalf("new primary lost preferences = %q", store.subtitleLanguages())
	}
}

func TestPrimarySubtitleLanguageReplacesSinglePreferenceAndFullPrimary(t *testing.T) {
	persisted := 0
	store := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguage: "en", SubtitleLanguages: []string{"en"}}, persist: func(string, any) error { persisted++; return nil }}
	if err := store.setPrimarySubtitleLanguage("fr"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(store.subtitleLanguages(), []string{"fr"}) || persisted != 1 {
		t.Fatalf("single preference was not replaced: languages=%q persisted=%d", store.subtitleLanguages(), persisted)
	}
	full := []string{"en", "fr", "de", "es", "it", "nl", "pl", "pt", "ru", "uk", "tr", "ar", "fa", "he", "hi", "bn", "ur", "id", "ms", "vi"}
	store.value.SubtitleLanguage, store.value.SubtitleLanguages, persisted = full[0], full, 0
	if err := store.setPrimarySubtitleLanguage("aa"); err != nil {
		t.Fatal(err)
	}
	want := append([]string{"aa"}, full[1:]...)
	if !slices.Equal(store.subtitleLanguages(), want) || persisted != 1 {
		t.Fatalf("full primary was not replaced: languages=%q persisted=%d", store.subtitleLanguages(), persisted)
	}
}

func TestSubtitleLanguagesMigrateLegacySettingAndSurviveRestart(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"subtitleLanguage":"es"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newSettingsStore("", dataDir, "", nil)
	if store.err != nil || !slices.Equal(store.subtitleLanguages(), []string{"es"}) {
		t.Fatalf("legacy setting: err=%v languages=%q", store.err, store.subtitleLanguages())
	}
	if err := store.setSubtitleLanguages([]string{"fr", "pt-BR", "zh-Hant"}); err != nil {
		t.Fatal(err)
	}
	reloaded := newSettingsStore("", dataDir, "", nil)
	if reloaded.err != nil || !slices.Equal(reloaded.subtitleLanguages(), []string{"fr", "pt-BR", "zh-Hant"}) {
		t.Fatalf("reloaded setting: err=%v languages=%q", reloaded.err, reloaded.subtitleLanguages())
	}
}

func TestSubtitleCoverageRequiresEveryPreferredLanguage(t *testing.T) {
	item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{"/media/Movie.srt", "/media/Movie.es.srt"}}
	ready, labels, missing := subtitleCoverageAll(item, []string{"en", "es", "fr"})
	if ready || !slices.Equal(labels, []string{"Default", "es"}) || !slices.Equal(missing, []string{"fr"}) {
		t.Fatalf("coverage = ready:%v labels:%q missing:%q", ready, labels, missing)
	}
	item.Subtitles = append(item.Subtitles, "/media/Movie.fr.srt")
	ready, _, missing = subtitleCoverageAll(item, []string{"en", "es", "fr"})
	if !ready || len(missing) != 0 {
		t.Fatalf("complete coverage = ready:%v missing:%q", ready, missing)
	}
}

func TestSubtitleCoverageDoesNotReuseOneVariantTrack(t *testing.T) {
	item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{"/media/Movie.pt-BR.srt"}}
	ready, _, missing := subtitleCoverageAll(item, []string{"pt", "pt-BR"})
	if ready || len(missing) != 1 {
		t.Fatalf("one variant covered two preferences: ready=%v missing=%q", ready, missing)
	}
}

func TestSubtitleCoverageLetsARegisteredVariantSatisfyItsBase(t *testing.T) {
	item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{"/media/Movie.en-US.srt"}}
	ready, _, missing := subtitleCoverageAll(item, []string{"en"})
	if !ready || len(missing) != 0 {
		t.Fatalf("regional track did not cover base: ready=%v missing=%q", ready, missing)
	}
}

func TestForcedSidecarDoesNotSatisfyFullDialogueCoverage(t *testing.T) {
	for _, path := range []string{"/media/Movie.en.forced.srt", "/media/Movie.forced.en.srt"} {
		item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{path}}
		ready, _, missing := subtitleCoverageAll(item, []string{"en"})
		if ready || !slices.Equal(missing, []string{"en"}) || subtitleLanguageFromPath(item.Path, path) != "en" || subtitleRoleFromPath(path) != "forced" {
			t.Errorf("forced-only sidecar covered full dialogue or lost its identity: %q ready=%v missing=%q", path, ready, missing)
		}
	}
}

func TestSubtitleCoverageKeepsProviderAliasesOutOfLocalSidecars(t *testing.T) {
	aliases := map[string]string{
		"ea": "es-419", "sp": "es-ES", "at": "ast", "pm": "pt-MZ", "zh_bg": "zh-Hant", "BR_PT": "pt-BR", "Brazillian Portuguese": "pt-BR", "Farsi_persian": "fa",
	}
	for suffix, requested := range aliases {
		item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{"/media/Movie." + suffix + ".srt"}}
		ready, labels, missing := subtitleCoverageAll(item, []string{requested})
		if ready || !slices.Equal(labels, []string{"Other"}) || !slices.Equal(missing, []string{requested}) {
			t.Errorf("local alias %q for %q = ready:%v labels:%q missing:%q", suffix, requested, ready, labels, missing)
		}
	}
	for suffix, requested := range map[string]string{
		"es-419": "es-419", "es-ES": "es-ES", "zh-Hans": "zh-Hans", "zh-Hant": "zh-Hant", "pt-BR": "pt-BR", "pt-PT": "pt-PT", "PT-br": "pt-BR",
	} {
		item := library.Item{Path: "/media/Movie.mkv", Subtitles: []string{"/media/Movie." + suffix + ".srt"}}
		ready, labels, missing := subtitleCoverageAll(item, []string{requested})
		if !ready || !slices.Equal(labels, []string{requested}) || len(missing) != 0 {
			t.Errorf("local tag %q for %q = ready:%v labels:%q missing:%q", suffix, requested, ready, labels, missing)
		}
	}
}

func TestEmbeddedToolsDoNotAdvertiseMissingItemLanguage(t *testing.T) { //nolint:cyclop // The table covers every embedded-tool language boundary.
	root := t.TempDir()
	item := library.Item{ID: "movie", Kind: "video", Path: filepath.Join(root, "Movie.mkv"), Title: "Movie"}
	if err := os.WriteFile(item.Path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "ffprobe")
	script := `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"en"}},{"index":1,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"ea"}}]}'
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil { //nolint:gosec // Test fixture must be executable.
		t.Fatal(err)
	}
	probe := newMediaProbe(executable)
	probe.ffmpeg = "ffmpeg"
	index := memoryLibraryIndex([]library.Item{item}, true)
	settings := newSettingsStore("", "", "", nil)
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), t.TempDir(), index, settings, "")
	manager := newSubtitleManager(index, settings, provider, probe)
	if language, found := manager.searchableSubtitleLanguage(t.Context(), item, []string{"aa"}); found {
		t.Fatalf("local-only language was advertised as %q", language)
	}
	if language, found := manager.searchableSubtitleLanguage(t.Context(), item, []string{"en"}); !found || language != "en" {
		t.Fatalf("embedded English was not advertised: %q, %v", language, found)
	}
	if language, found := manager.searchableSubtitleLanguage(t.Context(), item, []string{"es-419"}); found {
		t.Fatalf("provider alias escaped through embedded metadata as %q", language)
	}
	attempted, written, err := manager.fetchWantedLanguages(ownerRequest("/"), []string{"aa"}, 1)
	maintained := manager.maintainLanguageItems(t.Context(), []library.Item{item}, []string{"aa"}, 1, 0)
	if attempted != 0 || written != 0 || err != nil || maintained.Attempted != 0 || maintained.Failed != 0 {
		t.Fatalf("local-only work: attempted=%d written=%d error=%v maintained=%+v", attempted, written, err, maintained)
	}
}
