package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	defaultPageSize = 100
	maximumPageSize = 200
	maximumOffset   = 1_000_000
)

const DefaultPageSize = defaultPageSize

var ErrInvalidBrowse = errors.New("invalid library browse input")

type Candidate struct {
	Item    *library.Item
	Listed  bool
	Watched bool
	Updated time.Time
}

type BrowseAccess struct {
	ProgressMutex sync.Locker
	ListMutex     *sync.RWMutex
	Progress      *map[string]PlaybackState
	Listed        *map[string]bool
	ProfileID     string
	Owner         bool
	Visible       func(library.Item) bool
	Policy        *library.Policy
}

func (access BrowseAccess) WithPolicy(policy library.Policy) BrowseAccess {
	access.Policy, access.Visible = &policy, policy.Allows
	return access
}

func NewBrowseAccess(progressMutex sync.Locker, listMutex *sync.RWMutex, progress *map[string]PlaybackState, listed *map[string]bool, profileID string, owner bool, visible func(library.Item) bool) BrowseAccess {
	return BrowseAccess{ProgressMutex: progressMutex, ListMutex: listMutex, Progress: progress, Listed: listed, ProfileID: profileID, Owner: owner, Visible: visible}
}

type Letter struct {
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Offset  int    `json:"offset"`
	Href    string `json:"-"`
	Current bool   `json:"-"`
}

type Browse struct {
	values             url.Values
	query, view, order string
	letter, locale     string
	offset, limit      int
	titleOrdered       bool
}

type Result struct {
	Items                []library.Item
	Query, View, Sort    string
	Letter               string
	Letters              []Letter
	Total, Offset, Limit int
	references           []*library.Item
	pageStart, pageEnd   int
	values               url.Values
}

type sortReference struct {
	item *library.Item
	key  []byte
}

func ParseBrowse(values url.Values, locale string) (Browse, error) {
	for name := range values {
		if !oneOf(name, "q", "view", "sort", "offset", "limit", "lang", "letter") {
			return Browse{}, fmt.Errorf("%w: unknown library query", ErrInvalidBrowse)
		}
	}
	query, err := singleValue(values, "q", 512)
	if err != nil {
		return Browse{}, err
	}
	view, err := singleValue(values, "view", 32)
	if err != nil || !oneOf(view, "", "all", "list", "unwatched", "history", "movies", "shows", "collections", "playlists", "music", "audiobooks", "books", "photos") {
		return Browse{}, fmt.Errorf("%w: library view is invalid", ErrInvalidBrowse)
	}
	order, err := singleValue(values, "sort", 16)
	if err != nil || !oneOf(order, "", "title", "added", "year") {
		return Browse{}, fmt.Errorf("%w: library sort is invalid", ErrInvalidBrowse)
	}
	letter, err := parseLetter(values, locale)
	if err != nil || letter != "" && (query != "" || normalizeSort(order) != "title") {
		return Browse{}, fmt.Errorf("%w: library letter is invalid", ErrInvalidBrowse)
	}
	offset, err := browseInteger(values, "offset", 0, 0, maximumOffset)
	if err != nil {
		return Browse{}, err
	}
	limit, err := browseInteger(values, "limit", defaultPageSize, 1, maximumPageSize)
	if err != nil {
		return Browse{}, err
	}
	if view == "" {
		view = "all"
	}
	return Browse{values: cloneValues(values), query: query, view: view, order: order, letter: letter, locale: locale, offset: offset, limit: limit}, nil
}

func browseLibrary(ctx context.Context, values url.Values, locale string, load func(Browse) (browseSource, error), access func() BrowseAccess) (Result, error) {
	browse, err := ParseBrowse(values, locale)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	source, err := load(browse)
	browse.titleOrdered = source.ordered
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	grant := access()
	if source.visible != nil && grant.Policy != nil && browse.policyOnly() {
		order, err := source.visible(ctx, browse, *grant.Policy)
		if err != nil {
			return Result{}, err
		}
		return browse.result(ctx, order.items, order.letters)
	}
	return browse.ApplyAccess(ctx, source.references(), grant)
}

func ProfileProgress(values map[string]PlaybackState, profileID string, owner bool, itemID string) PlaybackState {
	return ViewerProgress(values, profileID, owner, itemID)
}

func (result Result) AllItems() []library.Item {
	items := make([]library.Item, len(result.references))
	for position, item := range result.references {
		items[position] = *item
	}
	return items
}

func (result Result) PreviousURL() string {
	return pageURL(result.values, max(result.pageStart, result.Offset-result.Limit), result.Limit, result.Offset > result.pageStart)
}

func (result Result) NextURL() string {
	return pageURL(result.values, result.Offset+result.Limit, result.Limit, result.Offset+result.Limit < result.pageEnd)
}

func viewMatches(candidate Candidate, view string) bool {
	switch view {
	case "list":
		return candidate.Listed
	case "unwatched":
		return !candidate.Watched
	case "history":
		return !candidate.Updated.IsZero()
	case "movies":
		return candidate.Item.Kind == "video" && candidate.Item.Show == ""
	case "shows":
		return candidate.Item.Kind == "video" && candidate.Item.Show != ""
	case "music":
		return candidate.Item.Kind == "audio"
	case "audiobooks", "books", "photos":
		return candidate.Item.Kind == strings.TrimSuffix(view, "s")
	default:
		return true
	}
}

func candidateReferences(candidates []Candidate) []*library.Item {
	items := make([]*library.Item, len(candidates))
	for position := range candidates {
		items[position] = candidates[position].Item
	}
	return items
}
