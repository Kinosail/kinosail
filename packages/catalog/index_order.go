package catalog

import (
	"context"
	"net/url"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/language"
)

const titleOrderReferenceBudget = 1 << 20

type titleOrder struct {
	items []*library.Item
	used  uint64
}

type titleOrderKey struct {
	locale, view string
}

func (index *Index) BrowseLibrary(ctx context.Context, values url.Values, locale string, access func() BrowseAccess) (Result, error) {
	return browseLibrary(ctx, values, locale, func(browse Browse) ([]*library.Item, bool, error) {
		key := browseTitleOrderKey(&browse, locale)
		if key.locale == "" {
			items, err := index.References()
			return items, false, err
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

func (index *Index) titleReferences(ctx context.Context, key titleOrderKey, locale string) ([]*library.Item, bool, error) {
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
		return nil, false, err
	}
	if entry != nil {
		return append([]*library.Item(nil), entry.items...), true, nil
	}
	if titleOrderCapacity(snapshot) == 0 || !snapshot.DistinctIDs() {
		return snapshotReferences(snapshot), false, nil
	}
	items := titleViewReferences(snapshot, key.view)
	if err := sortReferences(ctx, items, "title", "", locale); err != nil {
		return nil, false, err
	}
	index.rememberTitleOrder(key, version, items)
	return append([]*library.Item(nil), items...), true, nil
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

func (index *Index) rememberTitleOrder(key titleOrderKey, version uint64, items []*library.Item) {
	index.mu.Lock()
	defer index.mu.Unlock()
	capacity := titleOrderCapacity(index.current.Load())
	if capacity == 0 || version != index.titleOrderVersion || index.titleOrders[key] != nil {
		return
	}
	if index.titleOrders == nil {
		index.titleOrders = make(map[titleOrderKey]*titleOrder)
	}
	if len(index.titleOrders) == capacity {
		index.evictTitleOrder()
	}
	index.titleOrderUse++
	index.titleOrders[key] = &titleOrder{items: items, used: index.titleOrderUse}
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
