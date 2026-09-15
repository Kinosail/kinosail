package library

import (
	"slices"
	"strings"
)

var ratingLevel = map[string]int{"G": 1, "TV-Y": 1, "TV-Y7": 1, "TV-G": 1, "PG": 2, "TV-PG": 2, "PG-13": 3, "TV-14": 3, "R": 4, "TV-MA": 4, "NC-17": 5}

// Policy is one Viewer's library and maturity boundary.
type Policy struct {
	Owner     bool
	Rating    string
	Libraries []string
}

// NewPolicy creates one immutable Viewer policy.
func NewPolicy(owner bool, rating string, libraries []string) Policy {
	return Policy{Owner: owner, Rating: rating, Libraries: libraries}
}

// Allows reports whether one item is visible under this policy.
func (policy Policy) Allows(item Item) bool {
	return Allows(policy.Owner, policy.Rating, policy.Libraries, item)
}

// VisibilityIndex is the stable library subset needed for access filtering.
type VisibilityIndex interface {
	Snapshot() ([]Item, error)
	Find(string) (Item, bool)
	Safe(string) bool
}

// Visible returns the current items accepted by one Viewer policy.
func Visible(index VisibilityIndex, allow func(Item) bool) ([]Item, error) {
	items, err := index.Snapshot()
	visible := items[:0]
	for _, item := range items {
		if allow(item) {
			visible = append(visible, item)
		}
	}
	return visible, err
}

// VisibleItem resolves one safe item accepted by one Viewer policy.
func VisibleItem(index VisibilityIndex, id string, allow func(Item) bool) (Item, bool) {
	item, found := index.Find(id)
	return item, found && index.Safe(item.Path) && allow(item)
}

// Allows applies Kinosail's library and maturity policy to one item.
func Allows(owner bool, rating string, libraries []string, item Item) bool {
	if !owner && !slices.Contains(libraries, "all") && !slices.Contains(libraries, item.Library) {
		return false
	}
	if owner || rating == "all" {
		return true
	}
	level, rated := ratingLevel[strings.ToUpper(item.Rating)]
	return rated && level <= map[string]int{"": 2, "family": 2, "teen": 3}[rating]
}
