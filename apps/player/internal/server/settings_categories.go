package server

import (
	"regexp"
	"slices"
	"strings"
)

// Categories describe presentation only; forms, authorization, and saved values stay unchanged.
var settingsCategories = []struct {
	id, label, level, anchor string
	headings                 []string
}{
	{"general", "Overview", "general", "general", []string{"Server name", "Software updates", "Supporter passport", "Setup guide"}},
	{"playback", "Playback", "general", "playback", []string{"Playback", "Subtitles"}},
	{"household", "Profiles", "general", "profiles", []string{"Profiles"}},
	{"library", "Library", "general", "library", []string{"Library folders", "Movie artwork and details"}},
	{"appearance", "Appearance", "general", "appearance", []string{"Appearance", "Navigation", "Language"}},
	{"network", "Connections", "advanced", "access", []string{"Secure local access", "Jellyfin apps", "Trusted HTTPS on this network", "Trusted HTTPS (Required for Jellyfin apps)", "Remote access", "DLNA"}},
	{"security", "Security", "advanced", "security", []string{"MFA - Require extra sign-in protection for every Viewer Profile", "Automatic sign-out", "Secure sharing"}},
	{"integrations", "Integrations", "advanced", "settings-integrations", []string{"Integrations", "API keys"}},
	{"system", "Server", "advanced", "transcoder", []string{"Transcoder", "Video conversion", "Diagnostics", "Externally managed", "Library discovery", "Playback segment analysis", "Automatic maintenance"}},
	{"migration", "Migration", "advanced", "viewing-imports", []string{"Move viewing activity"}},
}

func categorizeSettingsPage(page string) string {
	slugPattern := regexp.MustCompile(`[^a-z0-9]+`)
	section := regexp.MustCompile(`<section([^>]*)><h2>([^<]+)</h2>`)
	page = section.ReplaceAllStringFunc(page, func(markup string) string {
		parts := section.FindStringSubmatch(markup)
		if !strings.Contains(parts[1], `data-settings-group=`) || strings.Contains(parts[1], `role="alert"`) {
			return markup
		}
		category := settingsCategoryFor(parts[2])
		attributes := parts[1] + ` data-settings-category="` + category + `"`
		if !strings.Contains(attributes, ` id="`) {
			slug := slugPattern.ReplaceAllString(strings.ToLower(parts[2]), "-")
			attributes += ` id="settings-` + strings.Trim(slug, "-") + `"`
		}
		return `<section` + attributes + `><h2>` + parts[2] + `</h2>`
	})
	// Keep setup available in Overview without placing it above the navigation.
	if start := strings.Index(page, `<section class="wide onboarding-settings"`); start >= 0 {
		if end := strings.Index(page[start:], `</section>`); end >= 0 {
			end += start + len(`</section>`)
			setup := page[start:end]
			page = page[:start] + page[end:]
			flow := `<div class="settings-flow" data-settings-flow>`
			page = strings.Replace(page, flow, flow+setup, 1)
		}
	}
	var nav strings.Builder
	nav.WriteString(`<nav class="settings-levels" data-settings-levels aria-label="Settings level"><a href="#general" data-settings-level="general">General</a><a href="#access" data-settings-level="advanced">Advanced</a></nav><nav class="settings-nav" data-settings-nav data-settings-organized aria-label="Settings categories">`)
	for _, entry := range settingsCategories {
		nav.WriteString(`<a data-settings-group="` + entry.id + `" data-settings-level="` + entry.level + `" href="#` + entry.anchor + `">` + entry.label + `</a>`)
	}
	nav.WriteString(`</nav>`)
	start := strings.Index(page, `<nav class="settings-nav"`)
	if start < 0 {
		return page
	}
	end := strings.Index(page[start:], `</nav>`)
	if end < 0 {
		return page
	}
	return page[:start] + nav.String() + page[start+end+len(`</nav>`):]
}

func settingsCategoryFor(heading string) string {
	for _, entry := range settingsCategories {
		if slices.Contains(entry.headings, heading) {
			return entry.id
		}
	}
	return "system"
}
