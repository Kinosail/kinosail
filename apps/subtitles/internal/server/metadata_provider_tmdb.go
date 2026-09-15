package server

import (
	"context"
	"errors"

	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

type metadataResult = sharedmetadata.Result

func (store *metadataStore) fetchRecord(ctx context.Context, item library.Item) (metadataRecord, error) {
	result, err := store.resolveRecord(ctx, item)
	if err != nil {
		return metadataRecord{}, err
	}
	return store.downloadMetadataImages(ctx, result)
}

func (store *metadataStore) resolveRecord(ctx context.Context, item library.Item) (metadataResult, error) {
	if store.configured() {
		result, err := store.resolveTMDBRecord(ctx, item)
		if err == nil {
			if store.tvmaze != nil {
				store.tvmaze.Fill(ctx, item, &result.Record)
			}
			return result, nil
		}
	}
	return store.resolveTVMazeRecord(ctx, item)
}

func (store *metadataStore) resolveTMDBRecord(ctx context.Context, item library.Item) (metadataResult, error) {
	return sharedmetadata.ResolveTMDBRecord(ctx, item, store.config.URL, store.getJSON, store.movieCollection, store.tvMetadata, store.artworkPath)
}

func (store *metadataStore) resolveEpisodeRecord(ctx context.Context, item library.Item, show metadataRecord) (metadataResult, error) {
	if !store.configured() {
		return store.resolveTVMazeRecord(ctx, item)
	}
	result, err := store.resolveTMDBEpisodeRecord(ctx, item, show)
	if err == nil && store.tvmaze != nil {
		store.tvmaze.Fill(ctx, item, &result.Record)
	}
	return result, err
}

func (store *metadataStore) resolveTMDBEpisodeRecord(ctx context.Context, item library.Item, show metadataRecord) (metadataResult, error) {
	return sharedmetadata.ResolveTMDBEpisode(ctx, item, show, store.tvMetadata, store.artworkPath)
}

func (store *metadataStore) downloadMetadataImages(ctx context.Context, result metadataResult) (metadataRecord, error) {
	failed := false
	for _, image := range result.Images {
		if store.downloadImage(ctx, image.Source, image.Target) == nil {
			continue
		}
		failed = true
		if image.Show {
			result.Record.ShowArtwork = ""
		} else {
			result.Record.Artwork = ""
		}
	}
	if failed {
		return result.Record, errors.New("metadata artwork could not be downloaded")
	}
	return result.Record, nil
}
