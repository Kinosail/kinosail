package server

import (
	"html"
	"regexp"
	"slices"
	"strings"
)

// Categories describe presentation only; forms, authorization, and saved values stay unchanged.
var settingsCategories = []struct {
	id, label, level, anchor string
	headings                 []string
}{
	{"playback", "Playback & subtitles", "general", "playback", []string{"Playback", "Subtitles"}},
	{"appearance", "Appearance & language", "general", "appearance", []string{"Appearance", "Navigation", "Language"}},
	{"library", "Library", "general", "library", []string{"Library folders", "Movie artwork and details", "Library discovery"}},
	{"household", "Viewer Profiles", "general", "profiles", []string{"Profiles"}},
	{"general", "General", "general", "general", []string{"Server name", "Software updates", "Supporter passport", "Setup guide"}},
	{"network", "Connections", "advanced", "access", []string{"Secure local access", "Jellyfin apps", "Trusted HTTPS on this network", "Trusted HTTPS (Required for Jellyfin apps)", "Watch away from home", "Remote access", "DLNA"}},
	{"security", "Security & sharing", "advanced", "security", []string{"Sign-in protection", "Automatic sign-out", "Secure sharing"}},
	{"integrations", "Integrations", "advanced", "settings-integrations", []string{"Integrations", "API keys"}},
	{"system", "Server tools", "advanced", "transcoder", []string{"Transcoder", "Video conversion", "Diagnostics", "Externally managed", "Playback segment analysis", "Automatic maintenance"}},
	{"migration", "Import viewing history", "advanced", "viewing-imports", []string{"Move viewing activity"}},
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
		for id, stableCategory := range settingsSectionCategories {
			if strings.Contains(parts[1], ` id="`+id+`"`) {
				category = stableCategory
				break
			}
		}
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
	nav.WriteString(`<nav class="settings-levels" data-settings-levels aria-label="Settings level"><a href="#playback" data-settings-level="general">Basic</a><a href="#access" data-settings-level="advanced">Advanced</a></nav><nav class="settings-nav" data-settings-nav data-settings-organized aria-label="Settings categories">`)
	for _, entry := range settingsCategories {
		nav.WriteString(`<a data-settings-group="` + entry.id + `" data-settings-level="` + entry.level + `" data-settings-description="` + settingsCategoryDescriptions[entry.id] + `" href="#` + entry.anchor + `">` + html.EscapeString(entry.label) + `</a>`)
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
	page = page[:start] + nav.String() + page[start+end+len(`</nav>`):]
	page = strings.Replace(page, `/static/main.kinosail.bundle.js?v=12`, `/static/main.kinosail.bundle.js?v=settings-2`, 1)
	page = strings.Replace(page, "Everyday preferences in General. Server tools and configuration in Advanced.", "Make Kinosail feel right for your household.", 1)
	return strings.Replace(page, `<div class="settings-flow" data-settings-flow>`, `<div class="settings-flow" data-settings-flow><p class="settings-category-description" data-settings-description hidden></p>`, 1)
}

func settingsCategoryFor(heading string) string {
	for _, entry := range settingsCategories {
		if slices.Contains(entry.headings, heading) {
			return entry.id
		}
	}
	return "system"
}

// Stable bookmarks take precedence over presentation copy.
var settingsSectionCategories = map[string]string{
	"general": "general", "updates": "general", "onboarding": "general",
	"playback": "playback", "appearance": "appearance", "navigation": "appearance",
	"profiles": "household", "library": "library", "system": "library",
	"security": "security", "session-timeouts": "security", "transcoder": "system",
	"access": "network", "jellyfin": "network", "trusted-https": "network", "viewing-imports": "migration",
}

var settingsCategoryDescriptions = map[string]string{
	"playback":     "Household playback defaults. Save each section to apply your changes.",
	"appearance":   "Choose your theme, language, and library navigation. Appearance is saved in this browser.",
	"library":      "Choose your media folders, keep them in sync, and add artwork and details.",
	"household":    "Give each person their own watch history, preferences, and library access.",
	"general":      "Name your Server, choose how it updates, or revisit setup.",
	"network":      "Connect other devices and choose how to reach your Server away from home.",
	"security":     "Manage sign-in protection, session limits, and shared media links.",
	"integrations": "Connect other services and manage API access to your Server.",
	"system":       "Tune video conversion, manage maintenance, and investigate Server issues.",
	"migration":    "Bring watched status and playback progress from another media server.",
}
