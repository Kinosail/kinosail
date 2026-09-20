package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCollectionSummariesSeparateDefaultAndCustomSources(t *testing.T) { //nolint:cyclop // One compact projection check covers source, count, and JSON parity.
	t.Parallel()
	store := newListStore("")
	servertest.CollectionSources(t, store.createCollection, store.setCollection, store.collectionSummaries)
}
