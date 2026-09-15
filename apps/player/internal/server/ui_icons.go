package server

import "html/template"

var uiIconNames = map[string]bool{
	"audiobook": true, "back": true, "book": true, "cast": true, "check": true, "collection": true, "music": true, "pip": true,
	"photo": true, "play": true, "playlist": true, "plus": true, "shows": true, "star": true,
}

func uiIcon(name string) template.HTML {
	if !uiIconNames[name] {
		return ""
	}
	return template.HTML(`<i class=i-` + name + ` aria-hidden=true></i>`) //nolint:gosec // The class suffix comes only from the fixed allowlist above.
}
