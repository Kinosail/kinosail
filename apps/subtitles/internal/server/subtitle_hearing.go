package server

import (
	"regexp"
	"strings"
)

var (
	subtitleHearingNotes   = regexp.MustCompile(`\[[^\]]{0,200}\]|\([^)]{0,200}\)`)
	subtitleHearingSpeaker = regexp.MustCompile(`^((?:<[ibu]>)*-?\s*)\p{Lu}[\p{Lu}\p{N} .'#-]{0,29}:\s+`)
)

func removeSubtitleHearingImpaired(cues []subtitleCue, sources [][]int) ([]subtitleCue, [][]int) {
	kept, keptSources := cues[:0], sources[:0]
	for index, cue := range cues {
		lines := strings.Split(subtitleHearingNotes.ReplaceAllString(cue.Text, ""), "\n")
		spoken := lines[:0]
		for _, line := range lines {
			line = strings.Join(strings.Fields(subtitleHearingSpeaker.ReplaceAllString(line, "$1")), " ")
			if strings.Trim(subtitleTags.ReplaceAllString(line, ""), " -♪♫#") != "" {
				spoken = append(spoken, line)
			}
		}
		if cue.Text = balanceSubtitleTags(strings.Join(spoken, "\n")); cue.Text == "" {
			continue
		}
		kept = append(kept, cue)
		if sources != nil {
			keptSources = append(keptSources, sources[index])
		}
	}
	return kept, keptSources
}
