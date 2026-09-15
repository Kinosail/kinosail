package markers

import (
	"math/bits"
	"sort"
)

const fingerprintPointSeconds = 4096.0 / 11025.0 / 3.0

type timeRange struct{ Start, End float64 }

func markerCandidatePairs(items []episodeFingerprint) [][2]int {
	if len(items) <= 64 {
		return allCandidatePairs(len(items))
	}
	audioIndex, visualIndex := make(map[uint32][]int), make(map[uint32][]int)
	supports := make(map[uint64]candidateSupport)
	for index, item := range items {
		indexCandidateKeys(audioIndex, supports, sampledAudioKeys(item), index, false)
		indexCandidateKeys(visualIndex, supports, sampledVisualKeys(item), index, true)
	}
	return supportedCandidatePairs(supports)
}

func sharedFingerprintSegment(left, right []uint32) (timeRange, timeRange, bool) {
	positions := make(map[uint32][]int, len(right))
	for index, point := range right {
		key := point & 0xfffffff0
		if len(positions[key]) < 8 {
			positions[key] = append(positions[key], index)
		}
	}
	shifts := make(map[int]int)
	for index, point := range left {
		for _, position := range positions[point&0xfffffff0] {
			shifts[position-index]++
		}
	}
	type candidate struct{ shift, seeds int }
	candidates := make([]candidate, 0, len(shifts))
	for shift, seeds := range shifts {
		if seeds >= 8 {
			candidates = append(candidates, candidate{shift, seeds})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].seeds > candidates[j].seeds })
	var bestLeft, bestRight timeRange
	for index, candidate := range candidates {
		if index == 32 {
			break
		}
		leftRange, rightRange := fingerprintRun(left, right, candidate.shift)
		if leftRange.End-leftRange.Start > bestLeft.End-bestLeft.Start {
			bestLeft, bestRight = leftRange, rightRange
		}
	}
	return bestLeft, bestRight, bestLeft.End-bestLeft.Start >= 20
}

func fingerprintRun(left, right []uint32, shift int) (timeRange, timeRange) {
	leftOffset, rightOffset := 0, 0
	if shift < 0 {
		leftOffset = -shift
	} else {
		rightOffset = shift
	}
	limit := min(len(left)-leftOffset, len(right)-rightOffset)
	start, last, matches, bestStart, bestLast, bestMatches := -1, -1, 0, -1, -1, 0
	for index := range limit {
		if bits.OnesCount32(left[index+leftOffset]^right[index+rightOffset]) <= 6 {
			if start < 0 || index-last > 28 {
				start, matches = index, 0
			}
			last, matches = index, matches+1
			if matches > bestMatches && float64(matches)/float64(last-start+1) >= .7 {
				bestStart, bestLast, bestMatches = start, last, matches
			}
		}
	}
	if bestStart < 0 {
		return timeRange{}, timeRange{}
	}
	return timeRange{float64(bestStart+leftOffset) * fingerprintPointSeconds, float64(bestLast+leftOffset+1) * fingerprintPointSeconds},
		timeRange{float64(bestStart+rightOffset) * fingerprintPointSeconds, float64(bestLast+rightOffset+1) * fingerprintPointSeconds}
}

func sharedVisualSegment(left, right []uint64) (timeRange, timeRange, bool) {
	positions := indexedVisualPositions(right)
	candidates := visualShiftCandidates(left, positions)
	bestLeft, bestRight := bestVisualRun(left, right, candidates)
	return bestLeft, bestRight, bestLeft.End-bestLeft.Start >= 20
}

func visualRun(left, right []uint64, shift int) (timeRange, timeRange) {
	leftOffset, rightOffset := 0, 0
	if shift < 0 {
		leftOffset = -shift
	} else {
		rightOffset = shift
	}
	limit := min(len(left)-leftOffset, len(right)-rightOffset)
	start, last, matches, bestStart, bestLast, bestMatches := -1, -1, 0, -1, -1, 0
	for index := range limit {
		if bits.OnesCount64(left[index+leftOffset]^right[index+rightOffset]) <= 8 {
			if start < 0 || index-last > 3 {
				start, matches = index, 0
			}
			last, matches = index, matches+1
			if matches > bestMatches && float64(matches)/float64(last-start+1) >= .7 {
				bestStart, bestLast, bestMatches = start, last, matches
			}
		}
	}
	if bestStart < 0 {
		return timeRange{}, timeRange{}
	}
	return timeRange{float64(bestStart + leftOffset), float64(bestLast + leftOffset + 1)},
		timeRange{float64(bestStart + rightOffset), float64(bestLast + rightOffset + 1)}
}

func mergePlaybackMarkers(authoritative, generated []Marker) []Marker {
	result, types := append([]Marker(nil), authoritative...), make(map[string]bool, len(authoritative))
	for _, marker := range authoritative {
		types[marker.Type] = true
	}
	for _, marker := range generated {
		if !types[marker.Type] {
			result = append(result, marker)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Start < result[j].Start })
	return result
}

func acceptCandidate(candidate *markerCandidate, value timeRange, duration float64, tail bool) {
	length := value.End - value.Start
	if length < 20 || length > 120 || !tail && value.End >= duration/2 || tail && duration-value.End > 120 {
		return
	}
	candidate.ranges = append(candidate.ranges, value)
}

func (candidate markerCandidate) consensus(minimum int) (timeRange, bool) {
	var best timeRange
	bestSupport := 0
	for _, pivot := range candidate.ranges {
		cluster := make([]timeRange, 0, len(candidate.ranges))
		for _, value := range candidate.ranges {
			overlap := min(pivot.End, value.End) - max(pivot.Start, value.Start)
			if overlap >= min(pivot.End-pivot.Start, value.End-value.Start)/2 {
				cluster = append(cluster, value)
			}
		}
		if len(cluster) < minimum {
			continue
		}
		shared := cluster[0]
		for _, value := range cluster[1:] {
			shared.Start, shared.End = max(shared.Start, value.Start), min(shared.End, value.End)
		}
		if length := shared.End - shared.Start; length >= 20 && (len(cluster) > bestSupport || len(cluster) == bestSupport && length > best.End-best.Start) {
			best, bestSupport = shared, len(cluster)
		}
	}
	return best, bestSupport >= minimum
}

func strongestRangeFromCandidates(audio, visual markerCandidate) (timeRange, string, bool) {
	audioRange, audioOK := audio.consensus(2)
	visualRange, visualOK := visual.consensus(2)
	return strongestRange(audioRange, audioOK, visualRange, visualOK)
}

func strongestRange(audio timeRange, audioOK bool, visual timeRange, visualOK bool) (timeRange, string, bool) {
	if audioOK {
		return audio, "fingerprint", true
	}
	if visualOK {
		return visual, "recurrence", true
	}
	return timeRange{}, "", false
}

func agreeingRange(audio timeRange, audioOK bool, visual timeRange, visualOK bool) (timeRange, bool) {
	if !audioOK || !visualOK {
		return timeRange{}, false
	}
	shared := timeRange{Start: max(audio.Start, visual.Start), End: min(audio.End, visual.End)}
	minimum := min(audio.End-audio.Start, visual.End-visual.Start) / 2
	return shared, shared.End-shared.Start >= max(20, minimum)
}
