package markers

import (
	"reflect"
	"testing"
)

func TestMatchingDenseAndNegativeShiftBoundaries(t *testing.T) {
	t.Parallel()
	dense := make([]uint32, 300)
	if _, _, ok := sharedFingerprintSegment(dense, dense); !ok {
		t.Fatal("dense shared fingerprint was rejected")
	}
	audio := sequence(200, 10)
	left, right := fingerprintRun(append(sequence(5, 9000), audio...), audio, -5)
	if left.End <= left.Start || right.End <= right.Start {
		t.Fatalf("negative audio shift = %#v / %#v", left, right)
	}
	if emptyLeft, emptyRight := fingerprintRun([]uint32{0}, []uint32{^uint32(0)}, 0); emptyLeft != (timeRange{}) || emptyRight != (timeRange{}) {
		t.Fatalf("unmatched audio = %#v / %#v", emptyLeft, emptyRight)
	}
	visual := visualSequence(30, 9000)
	left, right = visualRun(append(visualSequence(4, 1), visual...), visual, -4)
	if left.End <= left.Start || right.End <= right.Start {
		t.Fatalf("negative visual shift = %#v / %#v", left, right)
	}
	if emptyLeft, emptyRight := visualRun([]uint64{0}, []uint64{^uint64(0)}, 0); emptyLeft != (timeRange{}) || emptyRight != (timeRange{}) {
		t.Fatalf("unmatched visual = %#v / %#v", emptyLeft, emptyRight)
	}
}

func TestCandidateIndexBoundariesAndStableOrdering(t *testing.T) {
	t.Parallel()
	index := map[uint32][]int{7: {1}}
	supports := make(map[uint64]candidateSupport)
	indexCandidateKeys(index, supports, map[uint32]bool{7: true}, 2, false)
	pair := uint64(1)<<32 | 2
	if supports[pair].audio != 1 {
		t.Fatalf("audio support = %#v", supports)
	}
	pairs := supportedCandidatePairs(map[uint64]candidateSupport{
		uint64(3)<<32 | 4: {visual: 8},
		uint64(1)<<32 | 3: {audio: 8},
		uint64(1)<<32 | 2: {audio: 7},
	})
	if !reflect.DeepEqual(pairs, [][2]int{{1, 3}, {3, 4}}) {
		t.Fatalf("supported pairs = %#v", pairs)
	}
	positions := indexedVisualPositions([]uint64{0, ^uint64(0), 0x00ff00ff00ff00ff})
	if len(positions) == 0 {
		t.Fatal("useful visual fingerprint was not indexed")
	}
	if candidates := visualShiftCandidates([]uint64{0}, positions); len(candidates) != 0 {
		t.Fatalf("useless visual point produced candidates: %#v", candidates)
	}
}

func TestVisualCandidateRankingStopsAtBound(t *testing.T) {
	t.Parallel()
	visual := visualSequence(40, 9000)
	candidates := make([]visualShift, 33)
	for index := range candidates {
		candidates[index] = visualShift{shift: 0, seeds: len(candidates) - index}
	}
	left, right := bestVisualRun(visual, visual, candidates)
	if left.End-left.Start != 40 || right.End-right.Start != 40 {
		t.Fatalf("best visual range = %#v / %#v", left, right)
	}
	repeated := make([]uint64, 10)
	for index := range repeated {
		repeated[index] = 0x00ff00ff00ff00ff
	}
	shifts := visualShiftCandidates(repeated, indexedVisualPositions(repeated))
	if len(shifts) < 2 || shifts[0].seeds < shifts[1].seeds {
		t.Fatalf("ranked visual shifts = %#v", shifts)
	}
}
