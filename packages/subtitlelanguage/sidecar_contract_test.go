package subtitlelanguage

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestWrittenSidecarNamesReadBackWithTheSameLanguageAndRole(t *testing.T) {
	for _, media := range []string{"/media/Arrival (2016).mkv", "/media/Show/Season 01/Show.S01E01.1080p.mkv"} {
		for _, language := range []string{"en", "es", "pt-BR", "zh-Hans", "hi"} {
			for variant, role := range map[string]string{"": "", "sdh": "captions"} {
				path := library.SubtitleSidecarPath(media, language, variant)
				if got := FileLanguage(media, path); got != language {
					t.Errorf("%s reads back as language %q, want %q", path, got, language)
				}
				if got := library.SubtitleFileRole(path); got != role {
					t.Errorf("%s reads back as role %q, want %q", path, got, role)
				}
				if library.SubtitleFileForced(path) {
					t.Errorf("%s reads back as forced", path)
				}
				if got := library.SubtitleSidecarVariant(path); got != variant {
					t.Errorf("%s reads back as variant %q, want %q", path, got, variant)
				}
			}
		}
	}
}

func TestUnknownSidecarVariantWritesTheRegularName(t *testing.T) {
	if got := library.SubtitleSidecarPath("/media/Arrival.mkv", "en", "unknown"); got != "/media/Arrival.en.srt" {
		t.Fatalf("unknown variant path = %q", got)
	}
}
