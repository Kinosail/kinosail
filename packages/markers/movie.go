package markers

import (
	"context"
	"errors"

	"github.com/MikeO7/kinosail/packages/library"
)

func (analyzer *Analyzer) analyzeMovieOpenings(ctx context.Context, items []library.Item) error { //nolint:cyclop // Extraction, comparison, and consensus form one movie-library pass.
	titles := make([]episodeFingerprint, 0, len(items))
	for _, item := range items {
		media := analyzer.inspect(ctx, item)
		if !finite(media.Duration) || media.Duration <= 0 {
			return errors.New("playback segment media duration is unavailable")
		}
		length := min(media.Duration*.1, 300)
		opening, audioErr := analyzer.fingerprint(ctx, item, "opening", 0, length)
		openingVisual, visualErr := analyzer.visualFingerprint(ctx, item, "opening", 0, length)
		if audioErr != nil && visualErr != nil {
			return errors.New("playback opening analysis is unavailable")
		}
		titles = append(titles, episodeFingerprint{item: item, duration: media.Duration, opening: opening, openingVisual: openingVisual})
	}
	for _, pair := range markerCandidatePairs(titles) {
		if err := ctx.Err(); err != nil {
			return err
		}
		left, right := &titles[pair[0]], &titles[pair[1]]
		if leftRange, rightRange, ok := sharedFingerprintSegment(left.opening, right.opening); ok {
			acceptCandidate(&left.introAudio, leftRange, left.duration, false)
			acceptCandidate(&right.introAudio, rightRange, right.duration, false)
		}
		if leftRange, rightRange, ok := sharedVisualSegment(left.openingVisual, right.openingVisual); ok {
			acceptCandidate(&left.introVisual, leftRange, left.duration, false)
			acceptCandidate(&right.introVisual, rightRange, right.duration, false)
		}
	}
	for _, title := range titles {
		markers := make([]Marker, 0, 1)
		audio, audioOK := title.introAudio.consensus(2)
		visual, visualOK := title.introVisual.consensus(2)
		if intro, ok := agreeingRange(audio, audioOK, visual, visualOK); ok {
			markers = append(markers, Marker{Type: "intro", Label: "Intro", Start: intro.Start, End: intro.End, Source: "fingerprint"})
		} else if intro, _, ok := strongestRange(audio, audioOK, visual, visualOK); ok {
			markers = append(markers, Marker{Type: "intro", Label: "Intro", Start: intro.Start, End: intro.End, Source: "recurrence"})
		}
		analyzer.replaceDetected(title.item, []string{"intro"}, markers)
	}
	return ctx.Err()
}
