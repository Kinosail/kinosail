package subtitlelanguage

import "testing"

func TestFileLanguageReadsTheTagAfterTheMediaName(t *testing.T) {
	t.Parallel()
	for subtitle, want := range map[string]string{
		"/media/Film.en.srt":             "en",
		"/media/Film.EN.srt":             "en",
		"/media/Film.pt-BR.srt":          "pt-BR",
		"/media/Film.es-419.forced.srt":  "es-419",
		"/media/Film.zh-Hant.sdh.srt":    "zh-Hant",
		"/media/Film.English.srt":        "en",
		"/media/Film.Spanish.forced.srt": "es",
		"/media/Film.en.hi.srt":          "en",
		"/media/Film.srt":                "",
		"/media/Film.forced.srt":         "",
		"/media/Film.Directors.srt":      "",
		"/media/Other.en.srt":            "en",
	} {
		if got := FileLanguage("/media/Film.mkv", subtitle); got != want {
			t.Errorf("%s: language = %q, want %q", subtitle, got, want)
		}
	}
}
