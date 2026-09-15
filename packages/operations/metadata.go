package operations

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

const metadataWorkers = 4

// MetadataRefresh adapts provider and storage behavior to shared refresh coordination.
type MetadataRefresh struct {
	Available, Configured bool
	Items                 []library.Item
	Record                func(string) (metadata.Record, bool)
	Resolve               func(context.Context, library.Item) (metadata.Result, error)
	ResolveEpisode        func(context.Context, library.Item, metadata.Record) (metadata.Result, error)
	Download              func(context.Context, metadata.Result) (metadata.Record, error)
	Store                 func(map[string]metadata.Record) error
	RefreshLibrary        func(context.Context) error
}

// RefreshMetadata resolves missing records, persists partial success, and refreshes the library index.
func RefreshMetadata(ctx context.Context, config MetadataRefresh) error {
	if !config.Available {
		return errors.New("metadata provider is not configured")
	}
	if !validMetadataRefresh(ctx, config) {
		return errors.New("metadata refresh dependencies are incomplete")
	}
	updates, refreshErr := fetchMissing(ctx, config, missingMetadata(config))
	if len(updates) == 0 {
		return refreshErr
	}
	if err := config.Store(updates); err != nil {
		return err
	}
	if err := config.RefreshLibrary(ctx); err != nil {
		return err
	}
	return refreshErr
}

func validMetadataRefresh(ctx context.Context, config MetadataRefresh) bool {
	return ctx != nil && config.Record != nil && config.Resolve != nil && config.ResolveEpisode != nil && config.Download != nil && config.Store != nil && config.RefreshLibrary != nil
}

func missingMetadata(config MetadataRefresh) []library.Item {
	missing := make([]library.Item, 0)
	for _, item := range config.Items {
		record, found := config.Record(item.ID)
		if needsMetadata(item, record, found, config.Configured) {
			missing = append(missing, item)
		}
	}
	return missing
}

func fetchMissing(ctx context.Context, config MetadataRefresh, missing []library.Item) (map[string]metadata.Record, error) { //nolint:cyclop,gocognit // Grouping episodes and bounding independent provider work share one cancellation seam.
	if len(missing) == 0 {
		return nil, nil
	}
	groups := groupMetadata(missing)
	updates, slots := make(map[string]metadata.Record), make(chan struct{}, min(metadataWorkers, len(groups)))
	var updateMu sync.Mutex
	failed := 0
	var wait sync.WaitGroup
	for _, group := range groups {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wait.Wait()
			return nil, ctx.Err()
		}
		wait.Add(1)
		go func(group []library.Item) {
			defer func() { <-slots; wait.Done() }()
			groupUpdates, err := fetchMetadataGroup(ctx, config, group)
			updateMu.Lock()
			defer updateMu.Unlock()
			if err != nil {
				failed += len(group)
			}
			for id, record := range groupUpdates {
				updates[id] = record
			}
		}(group)
	}
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if failed > 0 {
		return updates, fmt.Errorf("metadata refresh failed for %d of %d items", failed, len(missing))
	}
	return updates, nil
}

func groupMetadata(items []library.Item) [][]library.Item {
	groups, positions := make([][]library.Item, 0, len(items)), make(map[string]int)
	for _, item := range items {
		key := item.ID
		if item.Show != "" {
			key = item.Library + "\x00" + item.Show
		}
		position, found := positions[key]
		if !found {
			position, positions[key] = len(groups), len(groups)
			groups = append(groups, nil)
		}
		groups[position] = append(groups[position], item)
	}
	return groups
}

func fetchMetadataGroup(ctx context.Context, config MetadataRefresh, group []library.Item) (map[string]metadata.Record, error) {
	results := make([]metadata.Result, len(group))
	first, err := config.Resolve(ctx, group[0])
	if err != nil {
		return nil, err
	}
	results[0] = first
	for position := 1; position < len(group); position++ {
		record, found := config.Record(group[position].ID)
		if found && metadata.Valid(record) {
			record.ShowTitle, record.ShowYear, record.ShowPlot, record.ShowArtwork = first.Record.ShowTitle, first.Record.ShowYear, first.Record.ShowPlot, first.Record.ShowArtwork
			record.ShowProviderIDs = maps.Clone(first.Record.ShowProviderIDs)
			record.ShowCast = append([]library.Person(nil), first.Record.ShowCast...)
			record.CastFetched = first.Record.CastFetched
			results[position] = metadata.Result{Record: record}
			continue
		}
		results[position], err = config.ResolveEpisode(ctx, group[position], first.Record)
		if err != nil {
			return nil, err
		}
	}
	updates := make(map[string]metadata.Record, len(group))
	showArtwork := ""
	var downloadErr error
	for position, item := range group {
		record, err := config.Download(ctx, results[position])
		if err != nil {
			downloadErr = err
		}
		if position == 0 {
			showArtwork = record.ShowArtwork
		} else if item.Show != "" {
			record.ShowArtwork = showArtwork
		}
		updates[item.ID] = record
	}
	return updates, downloadErr
}

func needsMetadata(item library.Item, record metadata.Record, found, configured bool) bool {
	artwork := item.Artwork
	if item.Show != "" {
		artwork = item.ShowArtwork
	}
	if item.Kind == "video" && (!found || !record.Owner && (!metadata.RegularFile(artwork) || configured && !record.CastFetched)) && (configured || metadata.TVMazeEligible(item)) {
		return true
	}
	return false
}
