// Package jellyfincompat implements shared Jellyfin identity compatibility.
package jellyfincompat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// NewID returns one random Jellyfin-compatible identifier.
func NewID() (string, error) { return IDFrom(rand.Reader) }

// IDFrom returns one Jellyfin-compatible identifier from exactly 128 random bits.
func IDFrom(reader io.Reader) (string, error) {
	data := make([]byte, 16)
	if _, err := io.ReadFull(reader, data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// ValidID reports whether a persisted Jellyfin server identifier is safe.
func ValidID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, character := range id {
		if !validIDCharacter(character) {
			return false
		}
	}
	return true
}

func validIDCharacter(character rune) bool {
	return character == '-' || character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

// EnsureID validates an existing identifier or generates one when absent.
func EnsureID(current string, generate func() (string, error)) (string, bool, error) {
	if current != "" {
		if !ValidID(current) {
			return "", false, errors.New("jellyfin server identifier is invalid")
		}
		return current, false, nil
	}
	id, err := generate()
	return id, err == nil, err
}

// LibraryRoutes lists the read routes granted by the Jellyfin library scope.
func LibraryRoutes() []string {
	return []string{
		"GET /System/Info", "GET /Library/MediaFolders", "GET /Users", "GET /Users/Me", "GET /Users/{id}",
		"GET /UserViews", "GET /Users/{user}/Views", "GET /Items", "GET /Users/{user}/Items", "GET /Items/Latest", "GET /Users/{user}/Items/Latest", "GET /Persons",
		"GET /Items/{id}", "GET /Users/{user}/Items/{id}", "GET /Shows/{id}/Seasons", "GET /Shows/{id}/Episodes",
		"GET /Items/{id}/Images/{type}", "GET /Items/{id}/Images/{type}/{index}",
	}
}

// App accepts the supported Seerr application names from one query value.
func App(values url.Values) (string, bool) {
	candidates := foldedValues(values, "App")
	if len(candidates) != 1 {
		return "", false
	}
	app := strings.TrimSpace(candidates[0])
	if len(app) > len("jellyseerr") {
		return "", false
	}
	switch strings.ToLower(app) {
	case "seerr", "jellyseerr":
		return "Seerr", true
	default:
		return "", false
	}
}

// ProviderIDs converts known local provider names to Jellyfin names.
func ProviderIDs(ids map[string]string) map[string]string {
	result := make(map[string]string)
	for provider, id := range ids {
		if strings.TrimSpace(id) != "" {
			name := map[string]string{"tmdb": "Tmdb", "themoviedb": "TheMovieDb", "imdb": "Imdb", "tvdb": "Tvdb", "anidb": "AniDB"}[strings.ToLower(provider)]
			if name != "" {
				result[name] = id
			}
		}
	}
	return result
}

// ShowProviderIDs returns the first episode-level show identity.
func ShowProviderIDs(episodes []library.Item) map[string]string {
	for _, episode := range episodes {
		if len(episode.ShowProviderIDs) > 0 {
			return ProviderIDs(episode.ShowProviderIDs)
		}
	}
	return map[string]string{}
}

// ItemIDs validates and normalizes one bounded ids query.
func ItemIDs(values url.Values, normalize func(string) string) (map[string]bool, error) {
	candidates := foldedValues(values, "ids")
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) != 1 {
		return nil, errors.New("invalid item ids")
	}
	parts := strings.Split(candidates[0], ",")
	if len(parts) > 100 {
		return nil, errors.New("invalid item ids")
	}
	result := make(map[string]bool, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || len(part) > 64 || strings.IndexFunc(part, invalidControl) >= 0 {
			return nil, errors.New("invalid item ids")
		}
		result[normalize(part)] = true
	}
	return result, nil
}

func foldedValues(values url.Values, name string) []string {
	result := make([]string, 0, 1)
	for key, candidates := range values {
		if strings.EqualFold(key, name) {
			result = append(result, candidates...)
		}
	}
	return result
}

func invalidControl(value rune) bool { return value < 0x20 || value == 0x7f }

type grant struct {
	ID, ProfileID, App, Secret string
	Expires                    time.Time
}

// Grants stores short-lived Seerr key handoffs.
type Grants struct{ values sync.Map }

// Store records one short-lived handoff.
func (grants *Grants) Store(profile, app, secret, id string, expires time.Time) {
	grants.values.Store(profile+"\x00"+app, grant{ID: id, ProfileID: profile, App: app, Secret: secret, Expires: expires})
}

// Items returns active handoffs for one profile and removes invalid state.
func (grants *Grants) Items(profile string, now time.Time) []map[string]any {
	items := make([]map[string]any, 0)
	grants.values.Range(func(key, value any) bool {
		grant, ok := value.(grant)
		if !ok || grant.Expires.Before(now) {
			grants.values.Delete(key)
			return true
		}
		if grant.ProfileID == profile {
			items = append(items, map[string]any{"Id": grant.ID, "AppName": grant.App, "AccessToken": grant.Secret})
		}
		return true
	})
	slices.SortFunc(items, func(left, right map[string]any) int {
		return strings.Compare(left["AppName"].(string), right["AppName"].(string))
	})
	return items
}
