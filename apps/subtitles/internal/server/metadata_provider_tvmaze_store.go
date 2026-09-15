package server

import (
	"context"
	"errors"

	"github.com/MikeO7/kinosail/packages/library"
)

func (store *metadataStore) resolveTVMazeRecord(ctx context.Context, item library.Item) (metadataResult, error) {
	if store.tvmaze == nil {
		return metadataResult{}, errors.New("metadata provider is not configured")
	}
	result, ok := store.tvmaze.Record(ctx, item)
	if !ok {
		return metadataResult{}, errors.New("metadata was not found")
	}
	return metadataResult{Record: result.Record}, nil
}
