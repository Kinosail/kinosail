package operations

import (
	"context"
	"maps"
	"reflect"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

const metadataAnchorAttempts = 3

type groupOutcome struct {
	updates  map[string]metadata.Record
	resolved []library.Item
	missed   []library.Item
	err      error
}

func (outcome groupOutcome) record(ctx context.Context, misses *metadata.Misses, group []library.Item) int {
	if ctx.Err() != nil {
		return 0
	}
	if outcome.err != nil {
		misses.Fail(group[0])
		return len(group)
	}
	for _, item := range outcome.resolved {
		misses.Clear(item)
	}
	for _, item := range outcome.missed {
		misses.FailItem(item)
	}
	return len(outcome.missed)
}

func fetchMetadataGroup(ctx context.Context, config MetadataRefresh, group []library.Item) groupOutcome {
	items, results, missed, err := resolveMetadataGroup(ctx, config, group)
	if err != nil {
		return groupOutcome{err: err}
	}
	updates, err := downloadMetadataGroup(ctx, config, items, results)
	return groupOutcome{updates: updates, resolved: items, missed: missed, err: err}
}

func resolveMetadataGroup(ctx context.Context, config MetadataRefresh, group []library.Item) ([]library.Item, []metadata.Result, []library.Item, error) {
	var missed []library.Item
	var err error
	for position := range min(len(group), metadataAnchorAttempts) {
		first, resolveErr := config.Resolve(ctx, group[position])
		if resolveErr != nil {
			missed, err = append(missed, group[position]), resolveErr
			continue
		}
		items, results := []library.Item{group[position]}, []metadata.Result{first}
		for _, item := range group[position+1:] {
			result, episodeErr := resolveGroupEpisode(ctx, config, item, first.Record)
			if episodeErr != nil {
				missed = append(missed, item)
				continue
			}
			items, results = append(items, item), append(results, result)
		}
		return items, results, missed, nil
	}
	return nil, nil, nil, err
}

func resolveGroupEpisode(ctx context.Context, config MetadataRefresh, item library.Item, show metadata.Record) (metadata.Result, error) {
	record, found := config.Record(item.ID)
	if !found || !metadata.Valid(record) || legacyStill(item, record, config.LegacyBackdrop) {
		return config.ResolveEpisode(ctx, item, show)
	}
	record.ShowTitle, record.ShowYear, record.ShowPlot, record.ShowArtwork, record.ShowBackdrop = show.ShowTitle, show.ShowYear, show.ShowPlot, show.ShowArtwork, show.ShowBackdrop
	record.BackdropChecked = show.BackdropChecked
	record.ShowProviderIDs = maps.Clone(show.ShowProviderIDs)
	record.ShowCast = append([]library.Person(nil), show.ShowCast...)
	record.CastFetched = show.CastFetched
	return metadata.Result{Record: record}, nil
}

func downloadMetadataGroup(ctx context.Context, config MetadataRefresh, group []library.Item, results []metadata.Result) (map[string]metadata.Record, error) {
	updates := make(map[string]metadata.Record, len(group))
	showArtwork, showBackdrop, backdropChecked := "", "", false
	var downloadErr error
	for position, item := range group {
		var missingImages []string
		for _, image := range results[position].Images {
			if config.replacesImage(image) {
				missingImages = append(missingImages, image.Target)
			}
		}
		record, err := results[position].Record, error(nil)
		if previous, found := config.Record(item.ID); !found || len(missingImages) > 0 || !reflect.DeepEqual(previous, record) {
			record, err = config.Download(ctx, results[position])
		}
		if err != nil {
			downloadErr = err
		}
		if position == 0 {
			showArtwork = record.ShowArtwork
			showBackdrop = record.ShowBackdrop
			backdropChecked = record.BackdropChecked
		} else if item.Show != "" {
			record.ShowArtwork = showArtwork
			record.ShowBackdrop = showBackdrop
			record.BackdropChecked = backdropChecked
		}
		if previous, found := config.Record(item.ID); found && reflect.DeepEqual(previous, record) && metadataApplied(item, record) && !slices.ContainsFunc(missingImages, metadata.RegularFile) {
			continue
		}
		updates[item.ID] = record
	}
	return updates, downloadErr
}
