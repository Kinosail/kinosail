package server

import (
	"regexp"
	"strings"
)

// Keep specialist playback choices available without leading the everyday form.
func simplifySettingsPlayback(page string) string {
	const explanation = "Direct First starts the original file on every connection. Wi-Fi speed, slow startup, and buffering never trigger conversion. After a confirmed format failure, Kinosail uses the smallest compatible change. Video transcoding always asks first."
	page = strings.Replace(page, "<p>"+explanation+"</p>", "", 1)
	choices := regexp.MustCompile(`<fieldset class="choice-set choice-cards"><legend>Default playback</legend>.*?</fieldset>`)
	page = choices.ReplaceAllStringFunc(page, func(form string) string {
		return `<details class="settings-compatibility" {{if ne .Playback "automatic"}}open{{end}}><summary>Playback compatibility · {{if eq .Playback "direct"}}Direct Play only{{else if eq .Playback "compatible"}}Compatibility first{{else}}Direct First{{end}}</summary><p>` + explanation + `</p>` + form + `</details>`
	})
	for action, label := range map[string]string{"/settings/playback": "Save playback preferences", "/settings/subtitles": "Save subtitle language", "/settings/server": "Save Server name"} {
		start := strings.Index(page, `<form action="`+action+`"`)
		if start < 0 {
			continue
		}
		end := strings.Index(page[start:], `</form>`)
		if end < 0 {
			continue
		}
		end += start
		page = page[:start] + strings.Replace(page[start:end], `<button>Save</button>`, `<button>`+label+`</button>`, 1) + page[end:]
	}
	return page
}
