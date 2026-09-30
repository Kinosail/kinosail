package catalog

import (
	"context"
	"net/url"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/language"
)

// Bound retained reference storage independently of library size and locale count.
const titleOrderReferenceBudget = 1 << 20

type titleOrder struct {
	items []*library.Item
	used  uint64
}

// BrowseLibrary reuses immutable title order while projecting current Viewer state.
func (index *Index) BrowseLibrary(ctx context.Context, values url.Values, locale string, access func() BrowseAccess) (Result, error) {
	var key string
	var version uint64
	var ordered bool
	result, err := browseLibrary(ctx, values, locale, func(browse Browse) ([]*library.Item, bool, error) {
		if browse.query != "" || normalizeSort(browse.order) != "title" || browse.view == "history" || len(locale) > 64 {
			items, err := index.References()
			return items, false, err
		}
		key = language.Make(locale).String()
		items, captured, cached, err := index.titleReferences(key)
		version, ordered = captured, cached
		return items, cached, err
	}, access)
	// Cold requests keep their existing selected sort. Only a complete catalog
	// projection can seed shared order; restricted viewers never add global work.
	if err == nil && key != "" && !ordered && result.View != "shows" && ctx.Err() == nil {
		index.rememberTitleOrder(key, version, result.references)
	}
	return result, err
}

func (index *Index) titleReferences(locale string) ([]*library.Item, uint64, bool, error) {
	index.mu.Lock()
	version := index.titleOrderVersion
	snapshot := index.items
	err := index.err
	if index.ready {
		err = nil
	}
	entry := index.titleOrders[locale]
	if entry != nil {
		index.titleOrderUse++
		entry.used = index.titleOrderUse
	}
	index.mu.Unlock()
	if entry != nil {
		return append([]*library.Item(nil), entry.items...), version, true, err
	}
	items := make([]*library.Item, len(snapshot))
	for position := range snapshot {
		items[position] = &snapshot[position]
	}
	return items, version, false, err
}

func (index *Index) rememberTitleOrder(locale string, version uint64, items []*library.Item) {
	index.mu.Lock()
	defer index.mu.Unlock()
	if version != index.titleOrderVersion || len(items) != len(index.items) || len(items) > titleOrderReferenceBudget || len(index.byID) != len(items) || index.titleOrders[locale] != nil {
		return
	}
	capacity := min(4, titleOrderReferenceBudget/max(1, len(items)))
	if index.titleOrders == nil {
		index.titleOrders = make(map[string]*titleOrder)
	}
	if len(index.titleOrders) == capacity {
		index.evictTitleOrder()
	}
	index.titleOrderUse++
	// Completed result references are immutable and never exposed as a slice.
	index.titleOrders[locale] = &titleOrder{items: items, used: index.titleOrderUse}
}

// evictTitleOrder removes the least recently read completed order under mu.
func (index *Index) evictTitleOrder() {
	var key string
	var oldest *titleOrder
	for locale, entry := range index.titleOrders {
		if oldest == nil || entry.used < oldest.used {
			key, oldest = locale, entry
		}
	}
	delete(index.titleOrders, key)
}
