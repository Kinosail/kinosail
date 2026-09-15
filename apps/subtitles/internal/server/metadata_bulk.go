package server

import (
	"context"
	"net/http"

	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

var (
	errMetadataSelection = sharedmetadata.ErrBulkSelection
	errMetadataFields    = sharedmetadata.ErrBulkFields
)

type metadataPatch = sharedmetadata.BulkPatch

var bulkMetadataView = newLocalizedTemplate("bulk-metadata", sharedmetadata.BulkMetadataHTML)

func (store *metadataStore) bulkEditor(index *libraryIndex) *sharedmetadata.BulkEditor {
	return sharedmetadata.NewBulkEditorFor(index, store.mu.RLocker(), &store.records, store.setAll, sharedmetadata.SubtitlesBulkPage(), bulkMetadataView.Execute, localizedError)
}

func (store *metadataStore) applyPatch(ctx context.Context, index *libraryIndex, ids []string, patch metadataPatch) (map[string]metadataRecord, error) {
	return store.bulkEditor(index).Apply(ctx, ids, patch)
}

func (store *metadataStore) register(mux *http.ServeMux, auth *authentication, index *libraryIndex) {
	bulk := store.bulkEditor(index)
	bulk.Register(mux, auth.owner)
	mux.Handle("POST /metadata/{id}/refresh", auth.owner(store.refreshHandler(index)))
	mux.Handle("POST /metadata/{id}", auth.owner(store.editHandler(index)))
}
