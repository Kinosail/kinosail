package catalog

import (
	"context"
	"net/url"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/language"
)

const titleOrderReferenceBudget = 1 << 20

type titleOrder struct {
	items   []*library.Item
	used    uint64
	visible map[string]*visibleOrder
}

type titleOrderKey struct {
	locale, view string
}

func (index *Index) BrowseLibrary(ctx context.Context, values url.Values, locale string, access func() BrowseAccess) (Result, error) {
	return browseLibrary(ctx, values, locale, func(browse Browse) (browseSource, error) {
		key := browseTitleOrderKey(&browse, locale)
		if key.locale == "" {
			items, err := index.References()
			return browseSource{items: items}, err
		}
		return index.titleReferences(ctx, key, locale)
	}, access)
}

func browseTitleOrderKey(browse *Browse, locale string) titleOrderKey {
	if browse.query != "" || normalizeSort(browse.order) != "title" || browse.view == "history" || len(locale) > 64 {
		return titleOrderKey{}
	}
	key := titleOrderKey{locale: language.Make(locale).String(), view: browse.view}
	if !oneOf(key.view, "movies", "music", "audiobooks", "books", "photos") {
		key.view = "all"
	}
	return key
}

func (index *Index) titleReferences(ctx context.Context, key titleOrderKey, locale string) (browseSource, error) {
	index.mu.Lock()
	version := index.titleOrderVersion
	snapshot := index.current.Load()
	err := index.err
	if index.ready {
		err = nil
	}
	entry := index.titleOrders[key]
	if entry == nil {
		entry = index.titleOrders[titleOrderKey{locale: key.locale, view: "all"}]
	}
	if entry != nil {
		index.titleOrderUse++
		entry.used = index.titleOrderUse
	}
	index.mu.Unlock()
	if err != nil {
		return browseSource{}, err
	}
	if entry != nil {
		return index.titleSource(entry), nil
	}
	if titleOrderCapacity(snapshot) == 0 || !snapshot.DistinctIDs() {
		return browseSource{items: snapshotReferences(snapshot)}, nil
	}
	items := titleViewReferences(snapshot, key.view)
	if err := sortReferences(ctx, items, "title", "", locale); err != nil {
		return browseSource{}, err
	}
	if entry := index.rememberTitleOrder(key, version, items); entry != nil {
		return index.titleSource(entry), nil
	}
	return browseSource{items: items, ordered: true}, nil
}

func titleViewReferences(snapshot *library.Snapshot, view string) []*library.Item {
	references := snapshotReferences(snapshot)
	if view == "all" {
		return references
	}
	selected := references[:0]
	for _, item := range references {
		if viewMatches(Candidate{Item: item}, view) {
			selected = append(selected, item)
		}
	}
	return selected
}

func titleOrderCapacity(snapshot *library.Snapshot) int {
	return min(4, titleOrderReferenceBudget/max(1, snapshot.Len()))
}

func (index *Index) rememberTitleOrder(key titleOrderKey, version uint64, items []*library.Item) *titleOrder {
	index.mu.Lock()
	defer index.mu.Unlock()
	capacity := titleOrderCapacity(index.current.Load())
	if capacity == 0 || version != index.titleOrderVersion {
		return nil
	}
	if existing := index.titleOrders[key]; existing != nil {
		return existing
	}
	if index.titleOrders == nil {
		index.titleOrders = make(map[titleOrderKey]*titleOrder)
	}
	if len(index.titleOrders) == capacity {
		index.evictTitleOrder()
	}
	index.titleOrderUse++
	entry := &titleOrder{items: items, used: index.titleOrderUse}
	index.titleOrders[key] = entry
	return entry
}

func (index *Index) evictTitleOrder() {
	var key titleOrderKey
	var oldest *titleOrder
	for locale, entry := range index.titleOrders {
		if oldest == nil || entry.used < oldest.used {
			key, oldest = locale, entry
		}
	}
	delete(index.titleOrders, key)
}
