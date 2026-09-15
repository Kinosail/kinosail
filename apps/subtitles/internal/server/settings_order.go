package server

import (
	"slices"
	"strings"
)

func orderSettingsPage(page string) string {
	page = strings.Replace(page,
		`<nav class="settings-nav" data-settings-nav aria-label="Settings sections"><a data-settings-group="general" href="#general">General</a><a data-settings-group="playback" href="#playback">Playback</a><a data-settings-group="access" href="#access">Access</a><a data-settings-group="migration" href="#viewing-imports">Migration</a><a data-settings-group="library" href="#library">Library</a><a data-settings-group="system" href="#system">System</a><a data-settings-group="appearance" href="#appearance">Appearance</a></nav>`,
		`<nav class="settings-nav" data-settings-nav aria-label="Settings sections"><a data-settings-group="playback" href="#playback">Playback</a><a data-settings-group="access" href="#access">Access</a><a data-settings-group="library" href="#library">Library</a><a data-settings-group="general" href="#security">General</a><a data-settings-group="appearance" href="#appearance">Appearance</a><a data-settings-group="system" href="#system">System</a><a data-settings-group="migration" href="#viewing-imports">Migration</a></nav>`, 1)
	flowStart := strings.Index(page, `<div class="settings-flow" data-settings-flow>`)
	flowEnd := strings.LastIndex(page, `</div></main>`)
	if flowStart < 0 || flowEnd <= flowStart {
		return page
	}
	contentStart := flowStart + len(`<div class="settings-flow" data-settings-flow>`)
	content := page[contentStart:flowEnd]
	firstSection := strings.Index(content, `<section`)
	if firstSection < 0 {
		return page
	}
	blocks := make([]settingsSectionBlock, 0, 32)
	prefix := content[:firstSection]
	blockStart, sectionStart := firstSection, firstSection
	var suffix string
	for sectionStart >= 0 {
		sectionEnd := strings.Index(content[sectionStart:], `</section>`)
		if sectionEnd < 0 {
			return page
		}
		sectionEnd += sectionStart + len(`</section>`)
		if strings.HasPrefix(content[sectionEnd:], `{{end}}`) {
			sectionEnd += len(`{{end}}`)
		}
		blocks = append(blocks, settingsSectionBlock{markup: content[blockStart:sectionEnd], rank: settingsGroupRank(content[sectionStart:sectionEnd])})
		next := strings.Index(content[sectionEnd:], `<section`)
		if next < 0 {
			suffix = content[sectionEnd:]
			break
		}
		blockStart, sectionStart = sectionEnd, sectionEnd+next
	}
	slices.SortStableFunc(blocks, func(first, second settingsSectionBlock) int { return first.rank - second.rank })
	ordered := strings.Builder{}
	ordered.Grow(len(content))
	ordered.WriteString(prefix)
	for _, block := range blocks {
		ordered.WriteString(block.markup)
	}
	ordered.WriteString(suffix)
	return page[:contentStart] + ordered.String() + page[flowEnd:]
}

type settingsSectionBlock struct {
	markup string
	rank   int
}

func settingsGroupRank(section string) int { //nolint:cyclop // The fixed display order is intentionally explicit.
	const attribute = `data-settings-group="`
	start := strings.Index(section, attribute)
	if start < 0 {
		return 99
	}
	start += len(attribute)
	end := strings.IndexByte(section[start:], '"')
	if end < 0 {
		return 99
	}
	switch section[start : start+end] {
	case "playback":
		return 0
	case "access":
		return 1
	case "library":
		return 2
	case "general":
		return 3
	case "appearance":
		return 4
	case "system":
		return 5
	case "migration":
		return 6
	default:
		return 99
	}
}
