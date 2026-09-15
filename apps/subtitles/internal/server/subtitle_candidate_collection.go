package server

import (
	"sort"

	"github.com/MikeO7/kinosail/packages/library"
)

func validSubDLResponse(response subDLResponse) bool {
	return response.Status && len(response.Results) > 0 && len(response.Results) <= 30 && len(response.Subtitles) > 0 && len(response.Subtitles) <= 30
}

func collectSubDLCandidates(item library.Item, language string, result subDLResult, subtitles []subDLSubtitle) ([]subtitleCandidate, bool) {
	candidates := make([]subtitleCandidate, 0, len(subtitles))
	for _, subtitle := range subtitles {
		found, valid := subDLCandidates(item, subtitle)
		if !valid {
			return nil, false
		}
		for _, candidate := range found {
			candidates = appendCandidate(candidates, item, language, result, candidate)
		}
	}
	return candidates, true
}

func sortSubDLCandidates(candidates []subtitleCandidate) {
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].Score == candidates[right].Score {
			return candidates[left].ReleaseMatch > candidates[right].ReleaseMatch
		}
		return candidates[left].Score > candidates[right].Score
	})
}
