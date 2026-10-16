package markers

import (
	"context"
	"errors"
	"sort"

	"github.com/MikeO7/kinosail/packages/library"
)

var errAnalysisCapacity = errors.New("playback segment analysis capacity is unavailable")

func (analyzer *Analyzer) itemFailed(ctx context.Context, item library.Item, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, errAnalysisCapacity) {
		return err
	}
	if analyzer.failed == nil {
		analyzer.failed = make(map[string]bool)
	}
	analyzer.failed[item.ID] = true
	return nil
}

func (analyzer *Analyzer) targeted(item library.Item) bool {
	return analyzer.targets == nil || analyzer.targets[item.ID]
}

func (analyzer *Analyzer) episodeFingerprints(ctx context.Context, item library.Item) (episodeFingerprint, error) {
	media := analyzer.inspect(ctx, item)
	if !finite(media.Duration) || media.Duration <= 0 {
		return episodeFingerprint{}, errors.New("playback segment media duration is unavailable")
	}
	openingLength := min(media.Duration*.25, 600)
	tailLength := min(media.Duration*.2, 900)
	opening, audioErr := analyzer.fingerprint(ctx, item, "opening", 0, openingLength)
	openingVisual, visualErr := analyzer.visualFingerprint(ctx, item, "opening", 0, openingLength)
	if audioErr != nil && visualErr != nil {
		return episodeFingerprint{}, errors.Join(errors.New("playback opening analysis is unavailable"), audioErr, visualErr)
	}
	tailOffset := media.Duration - tailLength
	tail, audioErr := analyzer.fingerprint(ctx, item, "tail", tailOffset, tailLength)
	tailVisual, visualErr := analyzer.visualFingerprint(ctx, item, "tail", tailOffset, tailLength)
	if audioErr != nil && visualErr != nil {
		return episodeFingerprint{}, errors.Join(errors.New("playback ending analysis is unavailable"), audioErr, visualErr)
	}
	return episodeFingerprint{item: item, duration: media.Duration, opening: opening, tail: tail, openingVisual: openingVisual, tailVisual: tailVisual, tailOffset: tailOffset}, nil
}

func (analyzer *Analyzer) analyzeSeason(ctx context.Context, items []library.Item) error {
	sort.Slice(items, func(i, j int) bool { return items[i].Episode < items[j].Episode })
	episodes := make([]episodeFingerprint, 0, len(items))
	for _, item := range items {
		episode, err := analyzer.episodeFingerprints(ctx, item)
		if err != nil {
			if err := analyzer.itemFailed(ctx, item, err); err != nil {
				return err
			}
			continue
		}
		episodes = append(episodes, episode)
	}
	for _, pair := range markerCandidatePairs(episodes) {
		if err := ctx.Err(); err != nil {
			return err
		}
		analyzer.comparePair(&episodes[pair[0]], &episodes[pair[1]])
	}
	for _, episode := range episodes {
		if !analyzer.targeted(episode.item) {
			continue
		}
		markers := make([]Marker, 0, 2)
		intro, source, ok := strongestRangeFromCandidates(episode.introAudio, episode.introVisual)
		if ok {
			markers = append(markers, Marker{Type: "intro", Label: "Intro", Start: intro.Start, End: intro.End, Source: source})
		}
		outro, source, ok := strongestRangeFromCandidates(episode.outroAudio, episode.outroVisual)
		if ok {
			markers = append(markers, Marker{Type: "outro", Label: "Outro", Start: outro.Start, End: outro.End, Source: source})
		}
		analyzer.replaceDetected(episode.item, []string{"intro", "outro"}, markers)
	}
	return ctx.Err()
}
