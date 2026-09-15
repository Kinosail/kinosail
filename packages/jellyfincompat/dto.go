package jellyfincompat

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// ItemOptions contains app-owned details for one shared Jellyfin item DTO.
type ItemOptions struct {
	CanDownload bool
	UserData    map[string]any
	MediaSource any
}

// ID expands one local item identifier into Jellyfin's 32-character form.
func ID(id string) string { return id + "0000000000000000" }

// RawID removes Kinosail's Jellyfin identifier suffix.
func RawID(id string) string {
	if len(id) == 32 && id[16:] == "0000000000000000" {
		return id[:16]
	}
	return id
}

// ShowID returns the stable local identifier for one series name.
func ShowID(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name)))
	return hex.EncodeToString(sum[:8])
}

// SeasonID returns the stable Jellyfin identifier for one series season.
func SeasonID(showID string, season int) string {
	encoded := strconv.FormatInt(int64(season), 16)
	return showID + strings.Repeat("0", 16-len(encoded)) + encoded
}

// Seasons returns the sorted season numbers in one series.
func Seasons(show library.Show) []int {
	set := make(map[int]bool)
	for _, item := range show.Episodes {
		set[item.Season] = true
	}
	seasons := make([]int, 0, len(set))
	for season := range set {
		seasons = append(seasons, season)
	}
	sort.Ints(seasons)
	return seasons
}

// FolderDTO projects one Jellyfin collection folder.
func FolderDTO(id, name, collection string) map[string]any {
	return map[string]any{"Id": id, "Name": name, "Type": "CollectionFolder", "CollectionType": collection, "IsFolder": true, "SortName": name}
}

// ItemDTO projects one library item with app-owned playback details.
func ItemDTO(item library.Item, options ItemOptions) map[string]any {
	kind, mediaType, parent := "Movie", "Video", MoviesID
	switch {
	case item.Show != "":
		kind, parent = "Episode", SeasonID(ShowID(item.Show), item.Season)
	case item.Kind == "audio":
		kind, mediaType, parent = "Audio", "Audio", ""
	case item.Kind == "photo":
		kind, mediaType, parent = "Photo", "Photo", ""
	}
	dto := map[string]any{
		"Id": ID(item.ID), "Name": item.Title, "SortName": item.Title, "Type": kind, "MediaType": mediaType,
		"IsFolder": false, "CanDelete": false, "CanDownload": options.CanDownload, "Container": strings.ToLower(item.Container),
		"ParentId": parent, "Overview": item.Plot, "OfficialRating": item.Rating, "DateCreated": item.Added.UTC().Format(time.RFC3339Nano),
		"UserData": options.UserData, "ProviderIds": ProviderIDs(item.ProviderIDs), "LocationType": "FileSystem", "HasSubtitles": len(item.Subtitles) > 0,
		"MediaSources": []any{options.MediaSource}, "MediaSourceCount": 1,
	}
	if item.Artwork != "" {
		dto["ImageTags"] = map[string]string{"Primary": ID(item.ID)}
	}
	if year, err := strconv.Atoi(item.Year); item.Year != "" && err == nil {
		dto["ProductionYear"] = year
	}
	if item.Genres != "" {
		dto["Genres"] = strings.Split(item.Genres, " · ")
	}
	if item.Show != "" {
		showID, showName := ShowID(item.Show), item.ShowTitle
		if showName == "" {
			showName = item.Show
		}
		dto["SeriesId"], dto["SeriesName"], dto["SeasonId"] = ID(showID), showName, SeasonID(showID, item.Season)
		dto["ParentIndexNumber"], dto["IndexNumber"] = item.Season, item.Episode
	}
	return dto
}

// SeriesDTO projects one series folder.
func SeriesDTO(show library.Show) map[string]any {
	id := ID(show.ID)
	dto := map[string]any{
		"Id": id, "Name": show.Title, "SortName": show.Title, "Type": "Series", "MediaType": "Video", "ParentId": ShowsID,
		"IsFolder": true, "ChildCount": len(show.Episodes), "ProviderIds": ShowProviderIDs(show.Episodes), "LocationType": "FileSystem",
		"UserData": map[string]any{"ItemId": id, "Key": id},
	}
	if show.ArtworkID != "" {
		dto["ImageTags"] = map[string]string{"Primary": show.ArtworkID}
	}
	return dto
}

// SeasonDTO projects one series season folder.
func SeasonDTO(show library.Show, season int) map[string]any {
	count, art := 0, ""
	for _, item := range show.Episodes {
		if item.Season == season {
			count++
			if art == "" {
				art = item.Artwork
			}
		}
	}
	id := SeasonID(show.ID, season)
	dto := map[string]any{
		"Id": id, "Name": "Season " + strconv.Itoa(season), "Type": "Season", "MediaType": "Video", "SeriesId": ID(show.ID),
		"SeriesName": show.Title, "ParentId": ID(show.ID), "IndexNumber": season, "IsFolder": true, "ChildCount": count,
		"UserData": map[string]any{"ItemId": id, "Key": id},
	}
	if art != "" {
		dto["ImageTags"] = map[string]string{"Primary": show.ArtworkID}
	}
	return dto
}

func seriesDTOs(shows []library.Show) []map[string]any {
	result := make([]map[string]any, 0, len(shows))
	for _, show := range shows {
		result = append(result, SeriesDTO(show))
	}
	return result
}
