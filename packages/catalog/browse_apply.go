package catalog

import (
	"context"
	"fmt"
	"sort"

	"github.com/MikeO7/kinosail/packages/library"
)

func (browse Browse) Apply(ctx context.Context, candidates []Candidate) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return browse.apply(ctx, append([]Candidate(nil), candidates...))
}

func (browse Browse) apply(ctx context.Context, candidates []Candidate) (Result, error) {
	selected, err := browseCandidates(ctx, candidates, browse.view, browse.query)
	if err != nil {
		return Result{}, err
	}
	if browse.view == "history" {
		sortHistory(selected)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return browse.applyItems(ctx, candidateReferences(selected))
}

func (browse Browse) applyItems(ctx context.Context, references []*library.Item) (Result, error) {
	items, err := browseReferences(ctx, references, browse)
	if err != nil {
		return Result{}, err
	}
	return browse.result(ctx, items, browseLetterBuckets(items, browse))
}

func (browse Browse) result(ctx context.Context, items []*library.Item, buckets []letterBucket) (Result, error) {
	letters := browseLetterLinks(browse.values, buckets, browse.letter)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	pageStart, pageEnd, offset, err := browse.pageWindow(items, letters)
	if err != nil {
		return Result{}, err
	}
	result := Result{Query: browse.query, View: browse.view, Sort: normalizeSort(browse.order), Letter: browse.letter, Letters: letters, Total: len(items), Offset: offset, Limit: browse.limit, references: items, pageStart: pageStart, pageEnd: pageEnd, values: cloneValues(browse.values)}
	result.Items = browsePage(items, offset, pageEnd, browse.limit)
	return result, nil
}

func browseCandidates(ctx context.Context, candidates []Candidate, view, query string) ([]Candidate, error) {
	selected := candidates[:0]
	normalized := searchText(query)
	for position, candidate := range candidates {
		if position%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if candidate.Item != nil && viewMatches(candidate, view) && (query == "" || matchesNormalized(*candidate.Item, normalized)) {
			selected = append(selected, candidate)
		}
	}
	return selected, ctx.Err()
}

func sortHistory(selected []Candidate) {
	sort.Slice(selected, func(left, right int) bool {
		return selected[left].Updated.After(selected[right].Updated) || selected[left].Updated.Equal(selected[right].Updated) && selected[left].Item.ID < selected[right].Item.ID
	})
}

func browseReferences(ctx context.Context, items []*library.Item, browse Browse) ([]*library.Item, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !browse.titleOrdered && !browse.historyOrdered() {
		if err := sortReferences(ctx, items, browse.order, browse.query, browse.locale); err != nil {
			return nil, err
		}
	}
	if browse.view == "shows" {
		items = showReferences(items)
		if err := sortReferences(ctx, items, browse.order, browse.query, browse.locale); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (browse Browse) historyOrdered() bool {
	return browse.view == "history" && browse.query == "" && normalizeSort(browse.order) == "title"
}

func browseLetterBuckets(items []*library.Item, browse Browse) []letterBucket {
	if browse.query == "" && normalizeSort(browse.order) == "title" && !browse.historyOrdered() {
		return letterBuckets(items, browse.locale)
	}
	return nil
}

func (browse Browse) pageWindow(items []*library.Item, letters []Letter) (int, int, int, error) {
	pageStart, pageEnd, offset := 0, len(items), browse.offset
	if browse.letter == "" {
		return pageStart, pageEnd, offset, nil
	}
	found := false
	for _, bucket := range letters {
		if bucket.Label == browse.letter {
			pageStart, pageEnd, found = bucket.Offset, bucket.Offset+bucket.Count, true
			break
		}
	}
	if !found || browse.values.Has("offset") && (offset < pageStart || offset >= pageEnd || (offset-pageStart)%browse.limit != 0) {
		return 0, 0, 0, fmt.Errorf("%w: library letter is invalid", ErrInvalidBrowse)
	}
	if !browse.values.Has("offset") {
		offset = pageStart
	}
	return pageStart, pageEnd, offset, nil
}

func browsePage(items []*library.Item, offset, pageEnd, limit int) []library.Item {
	if offset >= len(items) {
		return nil
	}
	page := items[offset:min(offset+limit, pageEnd)]
	result := make([]library.Item, len(page))
	for position, item := range page {
		result[position] = *item
	}
	return result
}

func itemCandidates(ctx context.Context, items []*library.Item, visible func(library.Item) bool, state func(string) (bool, PlaybackState)) ([]Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	candidates := make([]Candidate, 0, len(items))
	for position, item := range items {
		if position%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if item != nil && visible(*item) {
			listed, progress := state(item.ID)
			candidates = append(candidates, Candidate{Item: item, Listed: listed, Watched: progress.Watched, Updated: progress.Updated})
		}
	}
	return candidates, ctx.Err()
}

func (browse Browse) ApplyAccess(ctx context.Context, items []*library.Item, access BrowseAccess) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	access.ProgressMutex.Lock()
	access.ListMutex.RLock()
	var references []*library.Item
	var candidates []Candidate
	var err error
	if browse.view == "history" {
		candidates, err = itemCandidates(ctx, items, access.Visible, func(id string) (bool, PlaybackState) {
			return false, ProfileProgress(*access.Progress, access.ProfileID, access.Owner, id)
		})
	} else {
		references, err = browse.accessItems(ctx, items, access)
	}
	access.ListMutex.RUnlock()
	access.ProgressMutex.Unlock()
	if err != nil {
		return Result{}, err
	}
	if browse.view == "history" {
		return browse.apply(ctx, candidates)
	}
	references, err = searchReferences(ctx, references, browse.query)
	if err != nil {
		return Result{}, err
	}
	return browse.applyItems(ctx, references)
}

func (browse Browse) accessItems(ctx context.Context, items []*library.Item, access BrowseAccess) ([]*library.Item, error) {
	selected := make([]*library.Item, 0, len(items))
	for position, item := range items {
		if position%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if item == nil || !access.Visible(*item) {
			continue
		}
		candidate := Candidate{Item: item}
		switch browse.view {
		case "list":
			candidate.Listed = (*access.Listed)[access.ProfileID+":"+item.ID]
		case "unwatched":
			candidate.Watched = ProfileProgress(*access.Progress, access.ProfileID, access.Owner, item.ID).Watched
		}
		if viewMatches(candidate, browse.view) {
			selected = append(selected, item)
		}
	}
	return selected, ctx.Err()
}

func searchReferences(ctx context.Context, items []*library.Item, query string) ([]*library.Item, error) {
	if query == "" {
		return items, ctx.Err()
	}
	selected := items[:0]
	normalized := searchText(query)
	for position, item := range items {
		if position%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if matchesNormalized(*item, normalized) {
			selected = append(selected, item)
		}
	}
	return selected, ctx.Err()
}
