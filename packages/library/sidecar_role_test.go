package library

import "testing"

func TestSubtitleFileRoleAndForcedFlagFromTheFileName(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		role   string
		forced bool
	}{
		"Film.en.srt":                   {"", false},
		"Film.en.sdh.srt":               {"captions", false},
		"Film.en.cc.srt":                {"captions", false},
		"Film.en.hi.srt":                {"captions", false},
		"Film.hi.srt":                   {"", false},
		"Film.hi.forced.srt":            {"", true},
		"Film.EN.SDH.SRT":               {"captions", false},
		"Film.en.forced.srt":            {"", true},
		"Film.en.commentary.srt":        {"commentary", false},
		"Film.srt":                      {"", false},
		"/media/Show.S01E01.en.sdh.vtt": {"captions", false},
	} {
		if role, forced := SubtitleFileRole(name), SubtitleFileForced(name); role != want.role || forced != want.forced {
			t.Errorf("%s: role=%q forced=%t, want role=%q forced=%t", name, role, forced, want.role, want.forced)
		}
	}
}
