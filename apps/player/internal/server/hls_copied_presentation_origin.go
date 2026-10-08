package server

import "math"

const copiedHLSPrerollStrategy = "h264-idr-preroll-1"

// Decode remains the physical source IDR. RequestedMicros is presentation zero.
type copiedHLSPresentation struct {
	RequestedMicros int64
	Decode          copiedHLSKey
	Proof           *copiedHLSPresentationProof
}

// These fields describe generated track geometry. Structural validity alone
// never grants cache admission; generated assets require a separate certificate.
type copiedHLSPresentationProof struct {
	VideoPTS         float64
	VideoScale       int64
	VideoMediaTime   int64
	VideoDecodeTime  int64
	VideoComposition int64
}

func validCopiedHLSPresentation(timeline *copiedHLSTimeline) bool {
	mapping := timeline.Presentation
	if timeline.Strategy == "h264-idr-keys-1" {
		return mapping == nil
	}
	if !validCopiedHLSPresentationHeader(timeline) {
		return false
	}
	origin := float64(mapping.RequestedMicros) / 1_000_000
	if origin <= timeline.point(0) || origin >= timeline.segmentEnd(0) || origin >= timeline.End {
		return false
	}
	return mapping.Proof == nil || validCopiedHLSPresentationProof(timeline, origin)
}

func validCopiedHLSPresentationHeader(timeline *copiedHLSTimeline) bool {
	mapping := timeline.Presentation
	return timeline.Strategy == copiedHLSPrerollStrategy && mapping != nil && timeline.Clock == nil &&
		mapping.RequestedMicros > 0 && mapping.RequestedMicros <= 20_000_000 && mapping.Decode == timeline.Keys[0]
}

func validCopiedHLSPresentationTrack(proof *copiedHLSPresentationProof) bool {
	return proof.VideoScale > 0 && proof.VideoScale <= 1_000_000_000 &&
		proof.VideoDecodeTime == 0 && proof.VideoComposition >= 0 &&
		proof.VideoComposition <= proof.VideoScale && proof.VideoMediaTime > 0 &&
		proof.VideoMediaTime <= 16*proof.VideoScale
}

func validCopiedHLSPresentationProof(timeline *copiedHLSTimeline, origin float64) bool {
	proof := timeline.Presentation.Proof
	if !validCopiedHLSPresentationTrack(proof) || math.IsNaN(proof.VideoPTS) ||
		math.IsInf(proof.VideoPTS, 0) || proof.VideoPTS >= 0 || proof.VideoPTS < -15 {
		return false
	}
	physical := timeline.point(0) - origin
	generated := float64(proof.VideoComposition-proof.VideoMediaTime) / float64(proof.VideoScale)
	return math.Abs(proof.VideoPTS-physical) <= 0.000001 &&
		math.Abs(generated-physical) <= 0.000001
}

func copiedHLSPresentationBound(timeline *copiedHLSTimeline) bool {
	return timeline.Clock != nil || timeline.Presentation != nil && timeline.Presentation.Proof != nil
}

func copiedHLSPresentationPoint(timeline *copiedHLSTimeline, number int) float64 {
	if number == 0 && timeline.Presentation != nil {
		return float64(timeline.Presentation.RequestedMicros) / 1_000_000
	}
	return timeline.point(number)
}
