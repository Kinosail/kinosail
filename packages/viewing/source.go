package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Fetch reads and validates one bounded viewing snapshot from Plex or Jellyfin.
func Fetch(ctx context.Context, client *http.Client, input Input, lists bool) ([]Activity, error) {
	if input.Source == "plex" {
		return fetchPlexViewingActivity(ctx, client, input, lists)
	}
	return fetchJellyfinViewingActivity(ctx, client, input, lists)
}

func fetchJellyfinViewingActivity(ctx context.Context, client *http.Client, input Input, lists bool) ([]Activity, error) {
	userID, err := jellyfinViewingUserID(ctx, client, input)
	if err != nil {
		return nil, err
	}
	result := make([]Activity, 0)
	for start := 0; ; {
		page, pageErr := fetchJellyfinViewingPage(ctx, client, input, userID, start)
		if pageErr != nil {
			return nil, pageErr
		}
		result, pageErr = appendJellyfinViewingActivities(result, page.Items)
		if pageErr != nil {
			return nil, pageErr
		}
		if viewingPageComplete(len(page.Items), page.TotalRecordCount, start) {
			if lists {
				return addJellyfinPlaylists(ctx, client, input, userID, result)
			}
			return result, nil
		}
		start += len(page.Items)
		if start >= maximumViewingItems {
			return nil, errors.New("Jellyfin source returned too many items") //nolint:staticcheck // Jellyfin is a proper product name.
		}
	}
}

type jellyfinViewingPlaylist struct{ ID, Name, Type string }

func addJellyfinPlaylists(ctx context.Context, client *http.Client, input Input, userID string, activities []Activity) ([]Activity, error) { //nolint:cyclop,gocognit // Every source page is bounded and attached before matching.
	byID := make(map[string]int, len(activities))
	for index, activity := range activities {
		byID[activity.SourceID] = index
	}
	names, memberships := make(map[string]int), 0
	for start := 0; ; {
		query := url.Values{"UserId": {userID}, "Recursive": {"true"}, "IncludeItemTypes": {"Playlist"}, "EnableImages": {"false"}, "StartIndex": {strconv.Itoa(start)}, "Limit": {"200"}}
		var page struct {
			Items            []jellyfinViewingPlaylist
			TotalRecordCount int
		}
		if err := viewingSourceJSON(ctx, client, input, "/Items", query, &page); err != nil {
			return nil, err
		}
		if err := validateViewingPlaylistPage(len(page.Items), page.TotalRecordCount, start); err != nil {
			return nil, errors.New("Jellyfin source returned too many playlists") //nolint:staticcheck // Jellyfin is a proper product name.
		}
		for _, playlist := range page.Items {
			var err error
			activities, err = addJellyfinPlaylist(ctx, client, input, userID, playlist, names, byID, activities, &memberships)
			if err != nil {
				return nil, err
			}
		}
		if viewingPageComplete(len(page.Items), page.TotalRecordCount, start) {
			return activities, nil
		}
		start += len(page.Items)
		if start >= maximumViewingPlaylists {
			return nil, errors.New("Jellyfin source returned too many playlists") //nolint:staticcheck // Jellyfin is a proper product name.
		}
	}
}

func addJellyfinPlaylist(ctx context.Context, client *http.Client, input Input, userID string, playlist jellyfinViewingPlaylist, names map[string]int, byID map[string]int, activities []Activity, memberships *int) ([]Activity, error) { //nolint:cyclop,gocognit // One playlist is paged and projected at the adapter seam.
	if !strings.EqualFold(playlist.Type, "Playlist") {
		return activities, nil
	}
	name, nameErr := uniqueViewingPlaylistName(playlist.Name, names)
	if nameErr != nil || playlist.ID == "" || len(playlist.ID) > 512 {
		return nil, errors.New("Jellyfin source returned an invalid playlist") //nolint:staticcheck // Jellyfin is a proper product name.
	}
	for start := 0; ; {
		var page struct {
			Items            []struct{ ID string }
			TotalRecordCount int
		}
		itemQuery := url.Values{"UserId": {userID}, "StartIndex": {strconv.Itoa(start)}, "Limit": {"200"}}
		if err := viewingSourceJSON(ctx, client, input, "/Playlists/"+url.PathEscape(playlist.ID)+"/Items", itemQuery, &page); err != nil {
			return nil, err
		}
		if err := validateViewingPage(len(page.Items), page.TotalRecordCount); err != nil {
			return nil, errors.New("Jellyfin source returned invalid playlist pagination") //nolint:staticcheck // Jellyfin is a proper product name.
		}
		for offset, item := range page.Items {
			var itemErr error
			activities, itemErr = addViewingPlaylistItem(activities, byID, item.ID, name, "Jellyfin", start+offset, memberships)
			if itemErr != nil {
				return nil, itemErr
			}
		}
		if viewingPageComplete(len(page.Items), page.TotalRecordCount, start) {
			return activities, nil
		}
		start += len(page.Items)
		if start >= maximumViewingItems {
			return nil, errors.New("Jellyfin source playlist is too large") //nolint:staticcheck // Jellyfin is a proper product name.
		}
	}
}

func fetchPlexViewingActivity(ctx context.Context, client *http.Client, input Input, lists bool) ([]Activity, error) { //nolint:cyclop,funlen,gocognit // Library discovery and pagination stay together at the adapter boundary.
	var sections struct {
		MediaContainer struct{ Directory []struct{ Key, Type string } }
	}
	if err := viewingSourceJSON(ctx, client, input, "/library/sections", nil, &sections); err != nil {
		return nil, err
	}
	if len(sections.MediaContainer.Directory) > 1000 {
		return nil, errors.New("Plex source returned too many Library sections") //nolint:staticcheck // Plex and Library are proper product names.
	}
	result := make([]Activity, 0)
	for _, section := range sections.MediaContainer.Directory {
		var err error
		result, err = fetchPlexSection(ctx, client, input, section.Key, section.Type, result)
		if err != nil {
			return nil, err
		}
	}
	if lists {
		return addPlexPlaylists(ctx, client, input, result)
	}
	return result, nil
}

type plexViewingItem struct {
	RatingKey, Type, Title, GrandparentTitle string
	Guid                                     string `json:"guid"`
	Year, ParentIndex, Index, ViewCount      int
	ViewOffset, Duration                     int64
	LastViewedAt                             int64
	GuidList                                 []struct{ ID string } `json:"Guid"`
	Media                                    []struct{ Part []struct{ File string } }
}

type plexViewingPage struct {
	MediaContainer struct {
		TotalSize int
		Metadata  []plexViewingItem
	}
}

func fetchPlexSection(ctx context.Context, client *http.Client, input Input, key, kind string, result []Activity) ([]Activity, error) {
	typeID, err := plexSectionType(key, kind)
	if err != nil {
		return nil, err
	}
	if typeID == "" {
		return result, nil
	}
	seen := make(map[string]bool)
	for start := 0; ; {
		page, pageErr := fetchPlexViewingPage(ctx, client, input, key, typeID, start)
		if pageErr != nil {
			return nil, pageErr
		}
		var added bool
		result, added, pageErr = appendPlexViewingActivities(result, page.MediaContainer.Metadata, seen)
		if pageErr != nil {
			return nil, pageErr
		}
		count := len(page.MediaContainer.Metadata)
		if !added || viewingPageComplete(count, page.MediaContainer.TotalSize, start) {
			return result, nil
		}
		start += count
		if start >= maximumViewingItems {
			return nil, errors.New("Plex source returned too many items") //nolint:staticcheck // Plex is a proper product name.
		}
	}
}

func plexSectionType(key, kind string) (string, error) {
	if key == "" || len(key) > 512 {
		return "", errors.New("Plex source returned an invalid Library section") //nolint:staticcheck // Plex and Library are proper product names.
	}
	return map[string]string{"movie": "1", "show": "4"}[kind], nil
}

func fetchPlexViewingPage(ctx context.Context, client *http.Client, input Input, key, typeID string, start int) (plexViewingPage, error) {
	query := url.Values{"type": {typeID}, "includeGuids": {"1"}}
	headers := map[string]string{"X-Plex-Container-Start": strconv.Itoa(start), "X-Plex-Container-Size": "200"}
	var page plexViewingPage
	if err := viewingSourceJSONHeaders(ctx, client, input, "/library/sections/"+url.PathEscape(key)+"/all", query, headers, &page); err != nil {
		return plexViewingPage{}, err
	}
	if err := validateViewingPage(len(page.MediaContainer.Metadata), page.MediaContainer.TotalSize); err != nil {
		return plexViewingPage{}, err
	}
	return page, nil
}

func appendPlexViewingActivities(result []Activity, items []plexViewingItem, seen map[string]bool) ([]Activity, bool, error) {
	added := false
	for _, item := range items {
		if seen[item.RatingKey] {
			continue
		}
		seen[item.RatingKey] = true
		activity, err := plexViewingActivity(item)
		if err != nil {
			return nil, false, err
		}
		if len(result) >= maximumViewingItems {
			return nil, false, errors.New("Plex source returned too many items") //nolint:staticcheck // Plex is a proper product name.
		}
		result, added = append(result, activity), true
	}
	return result, added, nil
}

func plexViewingActivity(item plexViewingItem) (Activity, error) {
	if len(item.GuidList) > 32 || len(item.Media) > 8 {
		return Activity{}, errors.New("Plex source returned invalid item metadata") //nolint:staticcheck // Plex is a proper product name.
	}
	for _, media := range item.Media {
		if len(media.Part) > 16 {
			return Activity{}, errors.New("Plex source returned invalid item metadata") //nolint:staticcheck // Plex is a proper product name.
		}
	}
	path := ""
	if len(item.Media) > 0 && len(item.Media[0].Part) > 0 {
		path = item.Media[0].Part[0].File
	}
	updated, err := unixViewingTime(item.LastViewedAt)
	if err != nil {
		return Activity{}, err
	}
	return normalizeViewingActivity(Activity{SourceID: item.RatingKey, Kind: item.Type, Title: item.Title, Year: yearNumber(item.Year), Show: item.GrandparentTitle, Season: item.ParentIndex, Episode: item.Index, Path: path, ProviderIDs: plexProviderIDs(item.Guid, item.GuidList), Seconds: float64(item.ViewOffset) / 1000, Duration: float64(item.Duration) / 1000, Watched: item.ViewCount > 0, Updated: updated})
}
