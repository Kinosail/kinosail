package server

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// The existing synchronous theme asset projects the bookmarked category before
// body paint. Derive its anchors from the same catalog as the rendered sections.
func prepareSettingsInitial(source []byte) []byte {
	anchors := make(map[string]string)
	levels := make(map[string]string)
	slugPattern := regexp.MustCompile(`[^a-z0-9]+`)
	for _, entry := range settingsCategories {
		anchors[entry.anchor] = entry.id
		levels[entry.id] = entry.level
		for _, heading := range entry.headings {
			slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(heading), "-"), "-")
			anchors["settings-"+slug] = entry.id
		}
	}
	for anchor, category := range settingsSectionCategories {
		anchors[anchor] = category
	}
	anchorJSON, _ := json.Marshal(anchors)
	levelJSON, _ := json.Marshal(levels)
	anchorString, _ := json.Marshal(string(anchorJSON))
	levelString, _ := json.Marshal(string(levelJSON))
	source = bytes.ReplaceAll(source, []byte(`"KINOSAIL_SETTINGS_ANCHORS"`), anchorString)
	return bytes.ReplaceAll(source, []byte(`"KINOSAIL_SETTINGS_LEVELS"`), levelString)
}
