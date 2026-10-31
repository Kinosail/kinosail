package library

import (
	"path/filepath"
	"strings"

	"golang.org/x/text/language"
)

func SubtitleFileRole(path string) string {
	name := strings.ToLower(filepath.Base(path))
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), ".")
	if len(parts) >= 3 && parts[len(parts)-1] == "hi" {
		if tag, err := language.Parse(parts[len(parts)-2]); err == nil && tag != language.Und {
			return "captions"
		}
	}
	switch {
	case strings.Contains(name, ".sdh."), strings.Contains(name, ".cc."):
		return "captions"
	case strings.Contains(name, ".commentary."):
		return "commentary"
	default:
		return ""
	}
}

func SubtitleFileForced(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(path)), ".forced.")
}
