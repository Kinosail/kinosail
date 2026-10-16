package library

import (
	"slices"
	"strings"
)

var ratingLevel = map[string]int{"G": 1, "TV-Y": 1, "TV-Y7": 1, "TV-G": 1, "PG": 2, "TV-PG": 2, "PG-13": 3, "TV-14": 3, "R": 4, "TV-MA": 4, "NC-17": 5}

type Policy struct {
	Owner     bool
	Rating    string
	Libraries []string
}

func NewPolicy(owner bool, rating string, libraries []string) Policy {
	return Policy{Owner: owner, Rating: rating, Libraries: libraries}
}

func (policy Policy) Allows(item Item) bool {
	return Allows(policy.Owner, policy.Rating, policy.Libraries, item)
}

type VisibilityIndex interface {
	View(Policy) (*View, error)
	Find(string) (Item, bool)
	Safe(string) bool
}

func VisibleItem(index VisibilityIndex, id string, allow func(Item) bool) (Item, bool) {
	item, found := index.Find(id)
	return item, found && index.Safe(item.Path) && allow(item)
}

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
