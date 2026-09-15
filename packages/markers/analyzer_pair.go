package markers

func (analyzer *Analyzer) comparePair(left, right *episodeFingerprint) {
	if leftRange, rightRange, ok := sharedFingerprintSegment(left.opening, right.opening); ok {
		acceptCandidate(&left.introAudio, leftRange, left.duration, false)
		acceptCandidate(&right.introAudio, rightRange, right.duration, false)
	}
	if leftRange, rightRange, ok := sharedVisualSegment(left.openingVisual, right.openingVisual); ok {
		acceptCandidate(&left.introVisual, leftRange, left.duration, false)
		acceptCandidate(&right.introVisual, rightRange, right.duration, false)
	}
	if leftRange, rightRange, ok := sharedFingerprintSegment(left.tail, right.tail); ok {
		leftRange.Start, leftRange.End = leftRange.Start+left.tailOffset, leftRange.End+left.tailOffset
		rightRange.Start, rightRange.End = rightRange.Start+right.tailOffset, rightRange.End+right.tailOffset
		acceptCandidate(&left.outroAudio, leftRange, left.duration, true)
		acceptCandidate(&right.outroAudio, rightRange, right.duration, true)
	}
	if leftRange, rightRange, ok := sharedVisualSegment(left.tailVisual, right.tailVisual); ok {
		leftRange.Start, leftRange.End = leftRange.Start+left.tailOffset, leftRange.End+left.tailOffset
		rightRange.Start, rightRange.End = rightRange.Start+right.tailOffset, rightRange.End+right.tailOffset
		acceptCandidate(&left.outroVisual, leftRange, left.duration, true)
		acceptCandidate(&right.outroVisual, rightRange, right.duration, true)
	}
}
