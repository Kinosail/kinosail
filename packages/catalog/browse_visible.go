package catalog

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const maximumVisibleOrders = 8

type browseSource struct {
	items   []*library.Item
	ordered bool
	shared  bool
	visible func(context.Context, Browse, library.Policy) (*visibleOrder, error)
}

type visibleOrder struct {
	items   []*library.Item
	letters []letterBucket
	used    uint64
}

func (source browseSource) references() []*library.Item {
	if source.shared {
		return append([]*library.Item(nil), source.items...)
	}
	return source.items
}

func (browse Browse) policyOnly() bool {
	return browse.titleOrdered && browse.query == "" && normalizeSort(browse.order) == "title" && !oneOf(browse.view, "list", "unwatched", "history")
}

func (index *Index) titleSource(order *titleOrder) browseSource {
	return browseSource{items: order.items, ordered: true, shared: true, visible: func(ctx context.Context, browse Browse, policy library.Policy) (*visibleOrder, error) {
		return index.visibleTitles(ctx, order, browse, policy)
	}}
}

func (index *Index) visibleTitles(ctx context.Context, order *titleOrder, browse Browse, policy library.Policy) (*visibleOrder, error) {
	key := browse.view + "\x00" + browsePolicyKey(policy)
	index.mu.Lock()
	cached := order.visible[key]
	if cached != nil {
		index.titleOrderUse++
		cached.used = index.titleOrderUse
	}
	index.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	built, err := buildVisibleOrder(ctx, order.items, browse, policy)
	if err != nil {
		return nil, err
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	if existing := order.visible[key]; existing != nil {
		return existing, nil
	}
	if order.visible == nil {
		order.visible = make(map[string]*visibleOrder)
	}
	if len(order.visible) >= visibleOrderCapacity(len(order.items), titleOrderCapacity(index.current.Load())) {
		evictVisibleOrder(order.visible)
	}
	index.titleOrderUse++
	built.used = index.titleOrderUse
	order.visible[key] = built
	return built, nil
}

func buildVisibleOrder(ctx context.Context, ordered []*library.Item, browse Browse, policy library.Policy) (*visibleOrder, error) {
	selected := make([]*library.Item, 0, len(ordered))
	for position, item := range ordered {
		if position%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if item != nil && policy.Allows(*item) && viewMatches(Candidate{Item: item}, browse.view) {
			selected = append(selected, item)
		}
	}
	items, err := browseReferences(ctx, selected, browse)
	if err != nil {
		return nil, err
	}
	return &visibleOrder{items: slices.Clip(items), letters: letterBuckets(items, browse.locale)}, nil
}

func visibleOrderCapacity(items, orders int) int {
	return max(1, min(maximumVisibleOrders, titleOrderReferenceBudget/max(1, items*orders)))
}

func evictVisibleOrder(orders map[string]*visibleOrder) {
	oldest := ""
	for key, entry := range orders {
		if oldest == "" || entry.used < orders[oldest].used {
			oldest = key
		}
	}
	delete(orders, oldest)
}

func browsePolicyKey(policy library.Policy) string {
	if policy.Owner {
		return "owner"
	}
	libraries := slices.Clone(policy.Libraries)
	slices.Sort(libraries)
	var key strings.Builder
	key.WriteString(strconv.Quote(policy.Rating))
	for _, name := range slices.Compact(libraries) {
		key.WriteByte(' ')
		key.WriteString(strconv.Quote(name))
	}
	return key.String()
}
