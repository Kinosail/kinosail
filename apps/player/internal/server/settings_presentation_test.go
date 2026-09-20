package server

import (
	"strings"
	"testing"
)

func TestSettingsStableBookmarksOverridePresentationHeadings(t *testing.T) {
	for id, category := range settingsSectionCategories {
		page := categorizeSettingsPage(`<section id="` + id + `" data-settings-group="legacy"><h2>Renamed heading</h2><form action="/unchanged"><input name="value"></form></section>`)
		if !strings.Contains(page, `data-settings-category="`+category+`"`) || !strings.Contains(page, `<form action="/unchanged"><input name="value"></form>`) {
			t.Errorf("bookmark %s lost category or form: %s", id, page)
		}
	}
}

func TestSettingsPlaybackDisclosurePreservesControls(t *testing.T) {
	page := simplifySettingsPlayback(updateChoicePage(settingsHTML))
	for _, fragment := range []string{`<details class="settings-compatibility"`, `<summary>Playback compatibility`, `name="mode" value="automatic"`, `name="mode" value="direct"`, `name="mode" value="compatible"`, `name="subtitles"`, `name="markers"`, `Save playback preferences`, `Save subtitle language`, `Save Server name`} {
		if !strings.Contains(page, fragment) {
			t.Errorf("missing %q", fragment)
		}
	}
	if strings.Count(page, `class="settings-compatibility"`) != 1 {
		t.Fatal("compatibility disclosure must appear once")
	}
}
