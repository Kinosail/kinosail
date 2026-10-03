package server

import (
	"fmt"
	"math"
	"strings"
)

type inspectorQualityRow struct{ Name, Value string }
type inspectorCueColumn struct {
	Label, Timing, Warnings string
	Cue                     *subtitleReviewCue
}
type inspectorInitialView struct {
	Quality []inspectorQualityRow
	Cues    [][2]inspectorCueColumn
	Count   int
}

// Render the review already read for this request, using the interactive view's
// first page and normal escaping. Deferred refreshes retain the same geometry.
func initialInspector(review subtitleReview) inspectorInitialView {
	view := inspectorInitialView{}
	document := review.Proposed
	if document == nil {
		document = review.Current
	}
	if document != nil {
		role := "Dialogue subtitles"
		if review.Role == "captions" {
			role = "Declared accessibility captions"
		}
		quality := document.Quality
		view.Quality = []inspectorQualityRow{
			{"Source", review.Source}, {"Identity evidence", review.MatchEvidence}, {"Installed role", role},
			{"Cues", fmt.Sprint(quality.CueCount)}, {"Reading above 20 characters/sec", fmt.Sprint(quality.FastCues)},
			{"Overlapping cues", fmt.Sprint(quality.Overlaps)}, {"Lines above 42 characters", fmt.Sprint(quality.LongLines)},
			{"Timing evidence", quality.Timing}, {"Completeness", quality.Completeness},
		}
	}
	var current, proposed []subtitleReviewCue
	if review.Current != nil {
		current = review.Current.Cues
	}
	if review.Proposed != nil {
		proposed = review.Proposed.Cues
	}
	view.Count = max(len(current), len(proposed))
	for index := range min(view.Count, 40) {
		var columns [2]inspectorCueColumn
		for column, cues := range [][]subtitleReviewCue{current, proposed} {
			name := []string{"Current", "Proposed"}[column]
			columns[column] = inspectorCueColumn{Label: fmt.Sprintf("%s · cue %d", name, index+1)}
			if index < len(cues) {
				cue := &cues[index]
				stamp := func(seconds float64) string {
					return fmt.Sprintf("%d:%06.3f", int(math.Floor(seconds/60)), math.Mod(seconds, 60))
				}
				columns[column].Cue = cue
				columns[column].Timing = stamp(cue.Start) + " → " + stamp(cue.End)
				columns[column].Warnings = strings.Join(cue.Warnings, " · ")
			}
		}
		view.Cues = append(view.Cues, columns)
	}
	return view
}

const inspectorInitialCuesHTML = `{{range .Initial.Cues}}<div class="subtitle-cue-row">{{range .}}<div><small>{{.Label}}</small>{{if .Cue}}<button class="quiet" type="button" disabled>{{.Timing}}</button><p>{{.Cue.Text}}</p><small>{{.Warnings}}</small>{{else}}<p>—</p>{{end}}</div>{{end}}</div>{{end}}`
