package viewing

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/catalog"
)

const (
	maximumViewingSourceBody = 1 << 24
	maximumViewingItems      = 100000
	maximumViewingPlaylists  = 1000
	maximumViewingSeconds    = 31_622_400
)

func validateViewingPage(count, total int) error {
	if count < 0 {
		return errors.New("source returned invalid item pagination")
	}
	if count > 200 {
		return errors.New("source returned invalid item pagination")
	}
	if total < 0 {
		return errors.New("source returned invalid item pagination")
	}
	if total > maximumViewingItems {
		return errors.New("source returned invalid item pagination")
	}
	if total != 0 {
		if count > total {
			return errors.New("source returned invalid item pagination")
		}
	}
	return nil
}

func validateViewingPlaylistPage(count, total, start int) error {
	if validateViewingPage(count, total) != nil || total > maximumViewingPlaylists || count > maximumViewingPlaylists-start {
		return errors.New("source returned too many playlists")
	}
	return nil
}

func viewingPageComplete(count, total, start int) bool {
	if count == 0 {
		return true
	}
	if total > 0 {
		return start+count >= total
	}
	return count < 200
}

func normalizeViewingActivity(activity Activity) (Activity, error) { //nolint:cyclop // Remote activity fields form one trust boundary.
	if !validViewingIdentity(activity) {
		return Activity{}, errors.New("source returned invalid item identity")
	}
	if activity.Year != "" {
		year, err := strconv.Atoi(activity.Year)
		if err != nil || year < 1800 || year > 3000 {
			return Activity{}, errors.New("source returned invalid item year")
		}
	}
	if !validViewingProviderIDs(activity.ProviderIDs) {
		return Activity{}, errors.New("source returned invalid provider identifier")
	}
	if !validViewingPosition(activity.Seconds) || !validViewingPosition(activity.Duration) {
		return Activity{}, errors.New("source returned invalid resume position")
	}
	if activity.Duration > 0 {
		activity.Seconds = min(activity.Seconds, activity.Duration)
	}
	if activity.Watched {
		activity.Seconds = 0
	}
	return activity, nil
}

func validViewingIdentity(activity Activity) bool { //nolint:cyclop // Identity fields must satisfy all remote-data bounds together.
	return activity.SourceID != "" && len(activity.SourceID) <= 512 && activity.Title != "" && len(activity.Title) <= 512 && len(activity.Show) <= 512 && len(activity.Path) <= 4096 && activity.Season >= 0 && activity.Season <= 10000 && activity.Episode >= 0 && activity.Episode <= 100000 && len(activity.ProviderIDs) <= 16
}

func validViewingProviderIDs(ids map[string]string) bool {
	for provider, id := range ids {
		if len(provider) > 64 || len(id) > 512 {
			return false
		}
	}
	return true
}

func validViewingPosition(value float64) bool {
	return value >= 0 && value <= maximumViewingSeconds
}

func normalizeProviderIDs(ids map[string]string) map[string]string {
	result := make(map[string]string)
	for provider, id := range ids {
		provider, id = strings.ToLower(strings.TrimSpace(provider)), strings.TrimSpace(id)
		if (provider == "tmdb" || provider == "tvdb" || provider == "imdb") && id != "" {
			result[provider] = id
		}
	}
	return result
}

func plexProviderIDs(guid string, ids []struct{ ID string }) map[string]string {
	values := append([]struct{ ID string }{{guid}}, ids...)
	result := make(map[string]string)
	for _, value := range values {
		provider, id, found := strings.Cut(strings.TrimSpace(value.ID), "://")
		provider = strings.ToLower(provider)
		if found && (provider == "tmdb" || provider == "tvdb" || provider == "imdb") && id != "" {
			result[provider] = id
		}
	}
	return result
}

func parseViewingTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, errors.New("source returned invalid viewing timestamp")
	}
	return parsed, nil
}

func unixViewingTime(value int64) (time.Time, error) {
	if value < 0 {
		return time.Time{}, errors.New("source returned invalid viewing timestamp")
	}
	if value == 0 {
		return time.Time{}, nil
	}
	result := time.Unix(value, 0).UTC()
	if result.Year() > 3000 {
		return time.Time{}, errors.New("source returned invalid viewing timestamp")
	}
	return result, nil
}

func yearNumber(value int) string {
	if value > 0 {
		return strconv.Itoa(value)
	}
	return ""
}

func uniqueViewingPlaylistName(value string, seen map[string]int) (string, error) {
	name := strings.TrimSpace(value)
	if !catalog.ValidListName(name) {
		return "", errors.New("source returned an invalid playlist name")
	}
	countKey := "\x00" + name
	seen[countKey]++
	if seen[countKey] == 1 {
		if seen[name] == 0 {
			seen[name] = 1
			return name, nil
		}
	}
	base := name
	for range 1001 {
		suffix := " (" + strconv.Itoa(seen[countKey]) + ")"
		name = base
		maximum := 64 - len(suffix)
		for range len(name) {
			if len(name) > maximum {
				_, size := utf8.DecodeLastRuneInString(name)
				name = name[:len(name)-size]
			}
		}
		name += suffix
		if seen[name] == 0 {
			seen[name] = 1
			return name, nil
		}
		seen[countKey]++
	}
	return "", errors.New("source returned too many duplicate playlist names")
}

func addViewingPlaylistItem(activities []Activity, byID map[string]int, id, name, source string, position int, memberships *int) ([]Activity, error) {
	index, found := byID[id]
	if id == "" || len(id) > 512 {
		return nil, errors.New(source + " source returned an invalid playlist item")
	}
	if !found {
		if len(activities) >= maximumViewingItems {
			return nil, errors.New(source + " source returned too many items")
		}
		activities = append(activities, Activity{SourceID: id, Kind: "unsupported", Title: "Unsupported playlist item " + id})
		index, byID[id] = len(activities)-1, len(activities)-1
	}
	if activities[index].Playlists == nil {
		activities[index].Playlists = make(map[string]int)
	}
	if _, exists := activities[index].Playlists[name]; !exists {
		if *memberships >= maximumViewingItems {
			return nil, errors.New(source + " source returned too many playlist items")
		}
		*memberships++
	}
	activities[index].Playlists[name] = position
	return activities, nil
}
