package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCollectionSummariesSeparateDefaultAndCustomSources(t *testing.T) {
	t.Parallel()
	store := newListStore("")
	servertest.CollectionSources(t, store.createCollection, store.setCollection, store.collectionSummaries)
}
