package catalog

import (
	"fmt"
	"sort"

	"github.com/MikeO7/kinosail/packages/library"
)

// Apply selects, orders, groups, and pages Viewer-visible candidates.
func (browse Browse) Apply(candidates []Candidate) (Result, error) {
	selected := browseCandidates(candidates, browse.view, browse.query)
	if browse.view == "history" {
		sortHistory(selected)
	}
	items := browseReferences(selected, browse)
	letters := browseLetterGroups(items, browse)
	pageStart, pageEnd, offset, err := browse.pageWindow(items, letters)
	if err != nil {
		return Result{}, err
	}
	result := Result{Query: browse.query, View: browse.view, Sort: normalizeSort(browse.order), Letter: browse.letter, Letters: letters, Total: len(items), Offset: offset, Limit: browse.limit, references: items, pageStart: pageStart, pageEnd: pageEnd, values: cloneValues(browse.values)}
	result.Items = browsePage(items, offset, pageEnd, browse.limit)
	return result, nil
}

func browseCandidates(candidates []Candidate, view, query string) []Candidate {
	selected := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Item != nil && viewMatches(candidate, view) && (query == "" || Matches(*candidate.Item, query)) {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func sortHistory(selected []Candidate) {
	sort.Slice(selected, func(left, right int) bool {
		return selected[left].Updated.After(selected[right].Updated) || selected[left].Updated.Equal(selected[right].Updated) && selected[left].Item.ID < selected[right].Item.ID
	})
}

func browseReferences(selected []Candidate, browse Browse) []*library.Item {
	items := candidateReferences(selected)
	sortReferences(items, browse.order, browse.query, browse.locale)
	if browse.view == "shows" {
		items = showReferences(items)
		sortReferences(items, browse.order, browse.query, browse.locale)
	}
	return items
}

func browseLetterGroups(items []*library.Item, browse Browse) []Letter {
	if browse.query == "" && normalizeSort(browse.order) == "title" {
		return browseLetters(browse.values, items, browse.letter, browse.locale)
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

// ApplyItems projects app-owned visibility and Viewer state into one browse result.
func (browse Browse) ApplyItems(items []*library.Item, visible func(library.Item) bool, state func(string) (bool, PlaybackState)) (Result, error) {
	return browse.Apply(itemCandidates(items, visible, state))
}

func itemCandidates(items []*library.Item, visible func(library.Item) bool, state func(string) (bool, PlaybackState)) []Candidate {
	candidates := make([]Candidate, 0, len(items))
	for _, item := range items {
		if item != nil && visible(*item) {
			listed, progress := state(item.ID)
			candidates = append(candidates, Candidate{Item: item, Listed: listed, Watched: progress.Watched, Updated: progress.Updated})
		}
	}
	return candidates
}

// ApplyAccess reads one consistent app-owned Viewer state snapshot.
func (browse Browse) ApplyAccess(items []*library.Item, access BrowseAccess) (Result, error) {
	access.ProgressMutex.Lock()
	access.ListMutex.RLock()
	candidates := itemCandidates(items, access.Visible, func(id string) (bool, PlaybackState) {
		return (*access.Listed)[access.ProfileID+":"+id], ProfileProgress(*access.Progress, access.ProfileID, access.Owner, id)
	})
	access.ListMutex.RUnlock()
	access.ProgressMutex.Unlock()
	return browse.Apply(candidates)
}
