package metadata

import "testing"

func TestNormalizeConfigPreservesOverridesAndFillsDefaults(t *testing.T) {
	t.Parallel()
	defaults := NormalizeConfig(Config{})
	if defaults.URL != defaultAPIURL || defaults.ImageURL != defaultImageURL {
		t.Fatalf("defaults = %#v", defaults)
	}
	override := Config{URL: "https://api.example", ImageURL: "https://images.example", Token: "token", ChaptersURL: "https://chapters.example", TVMazeURL: "https://tv.example"}
	if got := NormalizeConfig(override); got != override {
		t.Fatalf("override = %#v", got)
	}
}
