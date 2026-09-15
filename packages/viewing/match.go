package viewing

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// SameState compares the source-controlled watched and resume fields.
func SameState(left, right catalog.PlaybackState) bool {
	return left.Watched == right.Watched && math.Abs(left.Seconds-right.Seconds) < .5
}

// Conflict reports whether the destination has newer meaningful state.
func Conflict(destination, source catalog.PlaybackState) bool {
	return !destination.Updated.IsZero() && (source.Updated.IsZero() || destination.Updated.After(source.Updated))
}

// Match resolves one activity by provider ID, semantic identity, then filename.
func Match(activity Activity, items []library.Item) (library.Item, string, string) { //nolint:cyclop // Conservative matching keeps every identity boundary in one decision.
	providerMatches := matchingItems(items, func(item library.Item) bool {
		if !sameKind(activity, item) {
			return false
		}
		for provider, id := range activity.ProviderIDs {
			if id != "" && strings.EqualFold(item.ProviderIDs[provider], id) {
				return true
			}
		}
		return false
	})
	if len(providerMatches) != 0 {
		return resolvedMatch(providerMatches, "provider identifier")
	}
	identityMatches := matchingItems(items, func(item library.Item) bool {
		return sameKind(activity, item) && sameIdentity(activity, item)
	})
	reason := "Movie title and year"
	if activity.Kind == "episode" {
		reason = "Show, season, and Episode"
	}
	switch len(identityMatches) {
	case 0:
		return library.Item{}, "unmatched", "no Kinosail Library item matched"
	case 1:
		return identityMatches[0], "matched", reason
	default:
		base := normalizedPathBase(activity.Path)
		pathMatches := matchingItems(identityMatches, func(item library.Item) bool { return base != "" && normalizedPathBase(item.Path) == base })
		if len(pathMatches) != 0 {
			return resolvedMatch(pathMatches, reason+" and media filename")
		}
		return resolvedMatch(identityMatches, reason)
	}
}

// ActivityKey returns a stable source observation key without credentials.
func ActivityKey(activity Activity) string {
	providers := make([]string, 0, len(activity.ProviderIDs))
	for provider, id := range activity.ProviderIDs {
		providers = append(providers, provider+":"+strings.ToLower(id))
	}
	sort.Strings(providers)
	if len(providers) > 0 {
		return activity.Kind + "|" + providers[0]
	}
	if path := normalizedPathBase(activity.Path); path != "" {
		return activity.Kind + "|path:" + path
	}
	return activity.Kind + "|" + normalizedText(activity.Show+activity.Title) + fmt.Sprintf("|%s|%d|%d", activity.Year, activity.Season, activity.Episode)
}

// Signature returns the source fields that trigger a sync update.
func Signature(activity Activity) string {
	return fmt.Sprintf("%t|%.3f", activity.Watched, activity.Seconds)
}

// Label returns a human-readable source item label.
func Label(activity Activity) string {
	if activity.Kind == "episode" {
		return fmt.Sprintf("%s · S%02dE%02d · %s", activity.Show, activity.Season, activity.Episode, activity.Title)
	}
	if activity.Year != "" {
		return activity.Title + " · " + activity.Year
	}
	return activity.Title
}

func resolvedMatch(items []library.Item, reason string) (library.Item, string, string) {
	if len(items) == 1 {
		return items[0], "matched", reason
	}
	return library.Item{}, "ambiguous", reason + " matched multiple Kinosail items"
}

func matchingItems(items []library.Item, match func(library.Item) bool) []library.Item {
	result := make([]library.Item, 0)
	for _, item := range items {
		if match(item) {
			result = append(result, item)
		}
	}
	return result
}

func sameKind(activity Activity, item library.Item) bool {
	if item.Kind != "video" {
		return false
	}
	if activity.Kind == "episode" {
		return item.Show != ""
	}
	return item.Show == ""
}

func sameIdentity(activity Activity, item library.Item) bool {
	if activity.Kind == "episode" {
		if normalizedText(activity.Show) != normalizedText(item.Show) || activity.Season != item.Season {
			return false
		}
		return activity.Episode == item.Episode
	}
	if activity.Year == "" {
		return false
	}
	if item.Year == "" {
		return false
	}
	if normalizedText(activity.Title) != normalizedText(item.Title) {
		return false
	}
	return activity.Year == item.Year
}

func normalizedText(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, value)
}

func normalizedPathBase(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	return strings.ToLower(strings.TrimSpace(value[strings.LastIndex(value, "/")+1:]))
}
