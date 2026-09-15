// Package catalog owns shared Library browsing, curation, and viewing state rules.
package catalog

import (
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

// DefaultPageSize is the canonical number of Library cards per page.
const DefaultPageSize = defaultPageSize

// ErrInvalidBrowse reports a malformed or unsupported Library query.
var ErrInvalidBrowse = errors.New("invalid library browse input")

// Candidate is one Viewer-visible Library item and its profile state.
type Candidate struct {
	Item    *library.Item
	Listed  bool
	Watched bool
	Updated time.Time
}

// BrowseAccess projects app-owned visibility and Viewer state under stable read locks.
type BrowseAccess struct {
	ProgressMutex *sync.Mutex
	ListMutex     *sync.RWMutex
	Progress      *map[string]PlaybackState
	Listed        *map[string]bool
	ProfileID     string
	Owner         bool
	Visible       func(library.Item) bool
}

// NewBrowseAccess binds app-owned profile state to the canonical browse projection.
func NewBrowseAccess(progressMutex *sync.Mutex, listMutex *sync.RWMutex, progress *map[string]PlaybackState, listed *map[string]bool, profileID string, owner bool, visible func(library.Item) bool) BrowseAccess {
	return BrowseAccess{ProgressMutex: progressMutex, ListMutex: listMutex, Progress: progress, Listed: listed, ProfileID: profileID, Owner: owner, Visible: visible}
}

// Letter is one locale-aware title jump in a Library result.
type Letter struct {
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Offset  int    `json:"offset"`
	Href    string `json:"-"`
	Current bool   `json:"-"`
}

// Browse is a validated Library query that can be applied after an index read.
type Browse struct {
	values             url.Values
	query, view, order string
	letter, locale     string
	offset, limit      int
}

// Result is one bounded, stable Library browse page.
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

// ParseBrowse strictly parses a Library query before any caller side effect.
func ParseBrowse(values url.Values, locale string) (Browse, error) { //nolint:cyclop // One parser owns the complete bounded browse vocabulary.
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
	return Browse{cloneValues(values), query, view, order, letter, locale, offset, limit}, nil
}

// BrowseLibrary validates a query before loading and projecting app-owned Library state.
func BrowseLibrary(values url.Values, locale string, load func() ([]*library.Item, error), access func() BrowseAccess) (Result, error) {
	browse, err := ParseBrowse(values, locale)
	if err != nil {
		return Result{}, err
	}
	items, err := load()
	if err != nil {
		return Result{}, err
	}
	return browse.ApplyAccess(items, access())
}

// ProfileProgress returns profile state with the owner's legacy fallback.
func ProfileProgress(values map[string]PlaybackState, profileID string, owner bool, itemID string) PlaybackState {
	state, found := values[profileID+":"+itemID]
	if !found && owner {
		state = values[itemID]
	}
	return state
}

// AllItems returns detached values for the complete selected result.
func (result Result) AllItems() []library.Item {
	items := make([]library.Item, len(result.references))
	for position, item := range result.references {
		items[position] = *item
	}
	return items
}

// PreviousURL returns the preceding page URL when it exists.
func (result Result) PreviousURL() string {
	return pageURL(result.values, max(result.pageStart, result.Offset-result.Limit), result.Limit, result.Offset > result.pageStart)
}

// NextURL returns the following page URL when it exists.
func (result Result) NextURL() string {
	return pageURL(result.values, result.Offset+result.Limit, result.Limit, result.Offset+result.Limit < result.pageEnd)
}

func viewMatches(candidate Candidate, view string) bool { //nolint:cyclop // The switch is the canonical browse-view vocabulary.
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
