package server

import (
	"encoding/json"
	"regexp"
	"strings"
)

// The existing synchronous theme asset projects the bookmarked category before
// body paint. Derive its anchors from the same catalog as the rendered sections.
func settingsInitialJS() []byte {
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
	return []byte(`if(location.pathname==="/settings"){(()=>{const anchors=` + string(anchorJSON) + `,levels=` + string(levelJSON) + `;let hash="";try{hash=decodeURIComponent(location.hash.slice(1));}catch{}const category=anchors[hash]||"playback";document.documentElement.dataset.settingsCategory=category;document.documentElement.dataset.settingsLevel=levels[category];if(hash)document.addEventListener("readystatechange",()=>{const target=document.getElementById(hash);if(!target)return;for(let ancestor=target;ancestor;ancestor=ancestor.parentElement)if(ancestor instanceof HTMLDetailsElement)ancestor.open=true;target.scrollIntoView({block:"start"});},{once:true});})();}`)
}
