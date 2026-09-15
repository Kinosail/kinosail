package markers

import (
	"math/bits"
	"sort"
)

type candidateSupport struct{ audio, visual int }

type visualShift struct{ shift, seeds int }

func allCandidatePairs(count int) [][2]int {
	pairs := make([][2]int, 0, count*(count-1)/2)
	for left := range count {
		for right := left + 1; right < count; right++ {
			pairs = append(pairs, [2]int{left, right})
		}
	}
	return pairs
}

func sampledAudioKeys(item episodeFingerprint) map[uint32]bool {
	keys := make(map[uint32]bool)
	for _, points := range [][]uint32{item.opening, item.tail} {
		for position, point := range points {
			if position%4 == 0 {
				keys[point&0xfffffff0] = true
			}
		}
	}
	return keys
}

func sampledVisualKeys(item episodeFingerprint) map[uint32]bool {
	keys := make(map[uint32]bool)
	for _, points := range [][]uint64{item.openingVisual, item.tailVisual} {
		for position, point := range points {
			if position%4 != 0 || !usefulVisualPoint(point) {
				continue
			}
			for band := range 4 {
				keys[visualBand(point, band)] = true
			}
		}
	}
	return keys
}

func indexCandidateKeys(index map[uint32][]int, supports map[uint64]candidateSupport, keys map[uint32]bool, item int, visual bool) {
	for key := range keys {
		for _, other := range index[key] {
			pair := uint64(other)<<32 | uint64(item) //nolint:gosec // Slice indexes are nonnegative and bounded by memory.
			value := supports[pair]
			if visual {
				value.visual++
			} else {
				value.audio++
			}
			supports[pair] = value
		}
		if len(index[key]) < 32 {
			index[key] = append(index[key], item)
		}
	}
}

func supportedCandidatePairs(supports map[uint64]candidateSupport) [][2]int {
	pairs := make([][2]int, 0, len(supports))
	for pair, value := range supports {
		if value.audio >= 8 || value.visual >= 8 {
			pairs = append(pairs, [2]int{int(pair >> 32), int(pair & 0xffffffff)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][0] < pairs[j][0] || pairs[i][0] == pairs[j][0] && pairs[i][1] < pairs[j][1]
	})
	return pairs
}

func indexedVisualPositions(points []uint64) map[uint32][]int {
	positions := make(map[uint32][]int, len(points)*4)
	for index, point := range points {
		if !usefulVisualPoint(point) {
			continue
		}
		for band := range 4 {
			key := visualBand(point, band)
			if len(positions[key]) < 16 {
				positions[key] = append(positions[key], index)
			}
		}
	}
	return positions
}

func visualShiftCandidates(points []uint64, positions map[uint32][]int) []visualShift {
	shifts := make(map[int]int)
	for index, point := range points {
		if !usefulVisualPoint(point) {
			continue
		}
		for band := range 4 {
			for _, position := range positions[visualBand(point, band)] {
				shifts[position-index]++
			}
		}
	}
	candidates := make([]visualShift, 0, len(shifts))
	for shift, seeds := range shifts {
		if seeds >= 20 {
			candidates = append(candidates, visualShift{shift, seeds})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].seeds > candidates[j].seeds })
	return candidates
}

func bestVisualRun(left, right []uint64, candidates []visualShift) (timeRange, timeRange) {
	var bestLeft, bestRight timeRange
	for index, candidate := range candidates {
		if index == 32 {
			break
		}
		leftRange, rightRange := visualRun(left, right, candidate.shift)
		if leftRange.End-leftRange.Start > bestLeft.End-bestLeft.Start {
			bestLeft, bestRight = leftRange, rightRange
		}
	}
	return bestLeft, bestRight
}

func usefulVisualPoint(point uint64) bool {
	count := bits.OnesCount64(point)
	return count >= 8 && count <= 56
}

func visualBand(point uint64, band int) uint32 {
	return uint32(band)<<16 | uint32(point>>uint(band*16)&0xffff) //nolint:gosec // Callers bound band to the four 16-bit fingerprint bands.
}
