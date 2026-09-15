package database

import "testing"

func TestLifecycleOpenPreservesSubtitlesValidation(t *testing.T) {
	t.Parallel()
	store, err := OpenContext(t.Context(), t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveJSON("settings.json", map[string]any{"subtitleLanguages": []string{"en"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSON("settings.json", map[string]any{"subtitleLanguages": []string{}}); err == nil {
		t.Fatal("lifecycle database accepted invalid settings")
	}
	var saved struct{ SubtitleLanguages []string }
	found, err := store.LoadJSON("settings.json", &saved)
	if err != nil || !found || len(saved.SubtitleLanguages) != 1 || saved.SubtitleLanguages[0] != "en" {
		t.Fatalf("rejected save changed settings: %#v %v %v", saved, found, err)
	}
}

func TestAppValidatorLeavesOtherDocumentsToSharedValidation(t *testing.T) {
	t.Parallel()
	if err := validateDocument("profiles.json", []byte("not settings")); err != nil {
		t.Fatalf("app validator applied settings rules to another document: %v", err)
	}
}
