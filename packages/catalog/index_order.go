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

type titleOrderKey struct {
	locale, view string
}

// BrowseLibrary reuses immutable title order while projecting current Viewer state.
func (index *Index) BrowseLibrary(ctx context.Context, values url.Values, locale string, access func() BrowseAccess) (Result, error) {
	var key titleOrderKey
	var version uint64
	var references []*library.Item
	var owner bool
	var ordered bool
	result, err := browseLibrary(ctx, values, locale, func(browse Browse) ([]*library.Item, bool, error) {
		if browse.query != "" || normalizeSort(browse.order) != "title" || browse.view == "history" || len(locale) > 64 {
			items, err := index.References()
			return items, false, err
		}
		key = titleOrderKey{locale: language.Make(locale).String(), view: browse.view}
		if !oneOf(key.view, "movies", "music", "audiobooks", "books", "photos") {
			key.view = "all"
		}
		items, captured, cached, err := index.titleReferences(key)
		version, ordered, references = captured, cached, items
		return items, cached, err
	}, func() BrowseAccess {
		viewer := access()
		owner = viewer.Owner
		return viewer
	})
	// Only a complete intrinsic projection can seed shared order. Every hit still
	// applies current visibility; restricted requests never add sorting work.
	if err == nil && key.locale != "" && !ordered && result.View != "shows" && ctx.Err() == nil {
		complete := len(references)
		if key.view != "all" {
			// Restricted cold requests keep the original projection work. Only
			// Owner results attempt new intrinsic admission, after profile locks.
			if !owner {
				return result, err
			}
			complete = countTitleView(ctx, references, key.view)
		}
		if ctx.Err() == nil {
			index.rememberTitleOrder(key, version, complete, result.references)
		}
	}
	return result, err
}

func countTitleView(ctx context.Context, references []*library.Item, view string) int {
	complete := 0
	for position, item := range references {
		if position%64 == 0 && ctx.Err() != nil {
			break
		}
		if viewMatches(Candidate{Item: item}, view) {
			complete++
		}
	}
	return complete
}

func (index *Index) titleReferences(key titleOrderKey) ([]*library.Item, uint64, bool, error) {
	index.mu.Lock()
	version := index.titleOrderVersion
	snapshot := index.items
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
	if entry != nil {
		return append([]*library.Item(nil), entry.items...), version, true, err
	}
	items := make([]*library.Item, len(snapshot))
	for position := range snapshot {
		items[position] = &snapshot[position]
	}
	return items, version, false, err
}

func (index *Index) rememberTitleOrder(key titleOrderKey, version uint64, complete int, items []*library.Item) {
	index.mu.Lock()
	defer index.mu.Unlock()
	capacity := min(4, titleOrderReferenceBudget/max(1, len(index.items)))
	if capacity == 0 || version != index.titleOrderVersion || len(items) != complete || len(index.byID) != len(index.items) || index.titleOrders[key] != nil {
		return
	}
	if index.titleOrders == nil {
		index.titleOrders = make(map[titleOrderKey]*titleOrder)
	}
	if len(index.titleOrders) == capacity {
		index.evictTitleOrder()
	}
	index.titleOrderUse++
	// Completed result references are immutable and never exposed as a slice.
	index.titleOrders[key] = &titleOrder{items: items, used: index.titleOrderUse}
}

// evictTitleOrder removes the least recently read completed order under mu.
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
