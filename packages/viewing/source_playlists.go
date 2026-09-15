package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type plexViewingPlaylist struct {
	RatingKey, Title, PlaylistType string
	Smart                          bool
}

func addPlexPlaylists(ctx context.Context, client *http.Client, input Input, activities []Activity) ([]Activity, error) { //nolint:cyclop,gocognit // Every source page is bounded and attached before matching.
	byID := make(map[string]int, len(activities))
	for index, activity := range activities {
		byID[activity.SourceID] = index
	}
	names, memberships := make(map[string]int), 0
	for start := 0; ; {
		var page struct {
			MediaContainer struct {
				TotalSize int
				Metadata  []plexViewingPlaylist
			}
		}
		headers := map[string]string{"X-Plex-Container-Start": strconv.Itoa(start), "X-Plex-Container-Size": "200"}
		if err := viewingSourceJSONHeaders(ctx, client, input, "/playlists", url.Values{"playlistType": {"video"}}, headers, &page); err != nil {
			return nil, err
		}
		if err := validateViewingPlaylistPage(len(page.MediaContainer.Metadata), page.MediaContainer.TotalSize, start); err != nil {
			return nil, errors.New("Plex source returned too many playlists") //nolint:staticcheck // Plex is a proper product name.
		}
		for _, playlist := range page.MediaContainer.Metadata {
			var err error
			activities, err = addPlexPlaylist(ctx, client, input, playlist, names, byID, activities, &memberships)
			if err != nil {
				return nil, err
			}
		}
		if viewingPageComplete(len(page.MediaContainer.Metadata), page.MediaContainer.TotalSize, start) {
			return activities, nil
		}
		start += len(page.MediaContainer.Metadata)
		if start >= maximumViewingPlaylists {
			return nil, errors.New("Plex source returned too many playlists") //nolint:staticcheck // Plex is a proper product name.
		}
	}
}

func addPlexPlaylist(ctx context.Context, client *http.Client, input Input, playlist plexViewingPlaylist, names map[string]int, byID map[string]int, activities []Activity, memberships *int) ([]Activity, error) { //nolint:cyclop,gocognit // One playlist is paged and projected at the adapter seam.
	name := strings.TrimSpace(playlist.Title)
	if playlist.Smart {
		name += " (snapshot)"
	}
	if playlist.PlaylistType != "" && playlist.PlaylistType != "video" {
		return activities, nil
	}
	var nameErr error
	name, nameErr = uniqueViewingPlaylistName(name, names)
	if nameErr != nil || playlist.RatingKey == "" || len(playlist.RatingKey) > 512 {
		return nil, errors.New("Plex source returned an invalid playlist") //nolint:staticcheck // Plex is a proper product name.
	}
	for start := 0; ; {
		var page struct {
			MediaContainer struct {
				TotalSize int
				Metadata  []struct{ RatingKey string }
			}
		}
		headers := map[string]string{"X-Plex-Container-Start": strconv.Itoa(start), "X-Plex-Container-Size": "200"}
		if err := viewingSourceJSONHeaders(ctx, client, input, "/playlists/"+url.PathEscape(playlist.RatingKey)+"/items", nil, headers, &page); err != nil {
			return nil, err
		}
		if err := validateViewingPage(len(page.MediaContainer.Metadata), page.MediaContainer.TotalSize); err != nil {
			return nil, errors.New("Plex source returned invalid playlist pagination") //nolint:staticcheck // Plex is a proper product name.
		}
		for offset, item := range page.MediaContainer.Metadata {
			var itemErr error
			activities, itemErr = addViewingPlaylistItem(activities, byID, item.RatingKey, name, "Plex", start+offset, memberships)
			if itemErr != nil {
				return nil, itemErr
			}
		}
		if viewingPageComplete(len(page.MediaContainer.Metadata), page.MediaContainer.TotalSize, start) {
			return activities, nil
		}
		start += len(page.MediaContainer.Metadata)
		if start >= maximumViewingItems {
			return nil, errors.New("Plex source playlist is too large") //nolint:staticcheck // Plex is a proper product name.
		}
	}
}
