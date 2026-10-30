package server

import "html/template"

var uiIconNames = map[string]bool{
	"audiobook": true, "back": true, "book": true, "bookmark": true, "cast": true, "check": true, "collection": true, "film": true,
	"history": true, "home": true, "music": true, "photo": true, "pip": true, "play": true, "playlist": true, "plus": true,
	"shows": true, "star": true, "unwatched": true,
}

func uiIcon(name string) template.HTML {
	if !uiIconNames[name] {
		return ""
	}
	return template.HTML(`<i class=i-` + name + ` aria-hidden=true></i>`) //nolint:gosec // The class suffix comes only from the fixed allowlist above.
}
