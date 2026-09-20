package server

import (
	"slices"
	"strings"
)

func orderSettingsPage(page string) string {
	page = strings.Replace(page, `<h2>Protected automatically</h2>`, `<h2>Sign-in protection</h2>`, 1)
	page = simplifySettingsPlayback(categorizeSettingsPage(page))
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

func settingsGroupRank(section string) int {
	for rank, category := range settingsCategories {
		if !strings.Contains(section, `data-settings-category="`+category.id+`"`) {
			continue
		}
		for index, heading := range category.headings {
			if strings.Contains(section, "<h2>"+heading+"</h2>") {
				return rank*100 + index
			}
		}
		return rank * 100
	}
	return 9999
}
