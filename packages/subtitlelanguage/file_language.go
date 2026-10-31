package subtitlelanguage

import (
	"path/filepath"
	"strings"
)

func FileLanguage(media, subtitle string) string {
	mediaName := strings.TrimSuffix(filepath.Base(media), filepath.Ext(media))
	name := strings.TrimSuffix(filepath.Base(subtitle), filepath.Ext(subtitle))
	suffix, matched := strings.CutPrefix(name, mediaName)
	if !matched {
		_, suffix, _ = strings.Cut(name, ".")
	}
	for part := range strings.SplitSeq(strings.TrimPrefix(suffix, "."), ".") {
		if canonical, ok := NormalizeFileName(part); ok {
			return canonical
		}
	}
	return ""
}
