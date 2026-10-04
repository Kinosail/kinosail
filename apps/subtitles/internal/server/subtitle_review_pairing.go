package server

type subtitleCueComparison struct {
	Current  []int  `json:"current,omitempty"`
	Proposed []int  `json:"proposed,omitempty"`
	Kind     string `json:"kind"`
}

func subtitleEditUsesCurrentCues(input subtitleEdit, review subtitleReview, data []byte) bool {
	return review.Current != nil && len(input.Data) == 0 && input.DraftID == "" && input.Text == "" && oneOf(input.Encoding, "", "auto") && subtitleFingerprint(data) == review.Fingerprint
}

func subtitleCueSources(count int) [][]int {
	sources := make([][]int, count)
	for index := range sources {
		sources[index] = []int{index}
	}
	return sources
}

func removeSubtitleEdgeCredits(cues []subtitleCue, sources [][]int) ([]subtitleCue, [][]int) {
	kept, keptSources := cues[:0], sources[:0]
	for index, cue := range cues {
		if index < 3 || index >= len(cues)-3 {
			cue.Text = removeSubtitleCredits(cue.Text)
		}
		if cue.Text == "" {
			continue
		}
		kept = append(kept, cue)
		if sources != nil {
			keptSources = append(keptSources, sources[index])
		}
	}
	return kept, keptSources
}

func subtitleReviewComparison(current, proposed *subtitleReviewDocument, sources [][]int) []subtitleCueComparison {
	currentCount := 0
	if current != nil {
		currentCount = len(current.Cues)
	}
	if sources == nil {
		return subtitleUnpairedComparison(currentCount, len(proposed.Cues))
	}
	rows := make([]subtitleCueComparison, 0, currentCount)
	cursor := 0
	for index, source := range sources {
		for cursor < source[0] {
			rows = append(rows, subtitleCueComparison{Current: []int{cursor}, Kind: "removed"})
			cursor++
		}
		kind := "matched"
		if len(source) > 1 {
			kind = "merged"
		}
		rows = append(rows, subtitleCueComparison{Current: source, Proposed: []int{index}, Kind: kind})
		cursor = source[0] + 1
		for _, original := range source[1:] {
			for cursor < original {
				rows = append(rows, subtitleCueComparison{Current: []int{cursor}, Kind: "removed"})
				cursor++
			}
			cursor = original + 1
		}
	}
	for cursor < currentCount {
		rows = append(rows, subtitleCueComparison{Current: []int{cursor}, Kind: "removed"})
		cursor++
	}
	return rows
}

func subtitleUnpairedComparison(currentCount, proposedCount int) []subtitleCueComparison {
	rows := make([]subtitleCueComparison, 0, currentCount+proposedCount)
	for index := range currentCount {
		rows = append(rows, subtitleCueComparison{Current: []int{index}, Kind: "unpaired-current"})
	}
	for index := range proposedCount {
		rows = append(rows, subtitleCueComparison{Proposed: []int{index}, Kind: "unpaired-proposed"})
	}
	return rows
}
