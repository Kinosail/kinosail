package markers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type markerGroups struct {
	seasons map[string][]library.Item
	movies  map[string][]library.Item
}

func (analyzer *Analyzer) analyze(ctx context.Context, items []library.Item) error {
	groups := groupMarkerItems(items)
	if err := analyzer.analyzeEpisodeGroups(ctx, groups.seasons); err != nil {
		return err
	}
	if err := analyzer.analyzeMovieGroups(ctx, groups.movies); err != nil {
		return err
	}
	return analyzer.analyzeCredits(ctx, items)
}

func groupMarkerItems(items []library.Item) markerGroups {
	groups := markerGroups{seasons: make(map[string][]library.Item), movies: make(map[string][]library.Item)}
	seen := make(map[string]bool)
	for _, item := range items {
		if item.Kind != "video" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		if item.Show != "" {
			key := strings.ToLower(item.Library + "\x00" + item.Show + "\x00" + strconv.Itoa(item.Season))
			groups.seasons[key] = append(groups.seasons[key], item)
			continue
		}
		key := strings.ToLower(item.Library)
		groups.movies[key] = append(groups.movies[key], item)
	}
	return groups
}

func (analyzer *Analyzer) analyzeEpisodeGroups(ctx context.Context, groups map[string][]library.Item) error {
	for _, episodes := range groups {
		if len(episodes) < 2 {
			continue
		}
		if err := analyzer.analyzeSeason(ctx, episodes); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (analyzer *Analyzer) analyzeMovieGroups(ctx context.Context, groups map[string][]library.Item) error {
	for _, titles := range groups {
		if len(titles) >= 3 {
			if err := analyzer.analyzeMovieOpenings(ctx, titles); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (analyzer *Analyzer) analyzeCredits(ctx context.Context, items []library.Item) error {
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.Kind != "video" {
			continue
		}
		media := analyzer.inspect(ctx, item)
		if !finite(media.Duration) || media.Duration <= 0 {
			return errors.New("playback segment media duration is unavailable")
		}
		if hasMarkerSource(media.Markers, "credits", "chapter") || hasMarkerSource(media.Markers, "credits", "manual") {
			continue
		}
		credits, err := analyzer.visualCredits(ctx, item, media.Duration)
		if err != nil {
			return errors.New("playback credit analysis is unavailable")
		}
		analyzer.replaceDetected(item, []string{"credits"}, credits)
	}
	return ctx.Err()
}
