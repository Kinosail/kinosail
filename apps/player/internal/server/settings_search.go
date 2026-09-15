package server

import "strings"

func addSettingsSearch(page string) string {
	search := `<section class="settings-search" data-settings-search aria-labelledby="settings-search-title"><h2 id="settings-search-title">Find a setting</h2><label for="settings-search-input">Search settings</label><input id="settings-search-input" data-settings-search-input type="search" autocomplete="off" autocapitalize="none" spellcheck="false" placeholder="Search by name or task…" aria-describedby="settings-search-help" aria-keyshortcuts="/"><p id="settings-search-help">Try “remote access”, “password”, “subtitles”, or “updates”.</p><output data-settings-search-status aria-live="polite"></output><div class="settings-search-results" data-settings-search-results hidden></div></section>`
	return strings.Replace(page, `<section class="wide onboarding-settings" id="onboarding"`, search+`<section class="wide onboarding-settings" id="onboarding"`, 1)
}
