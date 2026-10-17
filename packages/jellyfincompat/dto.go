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

type ItemOptions struct {
	CanDownload bool
	UserData    UserData
}

func ID(id string) string { return id + "0000000000000000" }

func RawID(id string) string {
	if len(id) == 32 && id[16:] == "0000000000000000" {
		return id[:16]
	}
	return id
}

func ShowID(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name)))
	return hex.EncodeToString(sum[:8])
}

func SeasonID(showID string, season int) string {
	encoded := strconv.FormatInt(int64(season), 16)
	return showID + strings.Repeat("0", 16-len(encoded)) + encoded
}

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

func FolderDTO(id, name, collection string) FolderRecord {
	return FolderRecord{ID: id, Name: name, Type: "CollectionFolder", CollectionType: collection, IsFolder: true, SortName: name}
}

func itemClass(item library.Item) (string, string, string) {
	switch {
	case item.Show != "":
		return "Episode", "Video", SeasonID(ShowID(item.Show), item.Season)
	case item.Kind == "audio":
		return "Audio", "Audio", ""
	case item.Kind == "photo":
		return "Photo", "Photo", ""
	}
	return "Movie", "Video", MoviesID
}

func ItemDTO(item library.Item, options ItemOptions) ItemRecord {
	kind, mediaType, parent := itemClass(item)
	id := ID(item.ID)
	record := ItemRecord{
		ID: id, Name: item.Title, SortName: item.Title, Type: kind, MediaType: mediaType, CanDownload: options.CanDownload,
		Container: strings.ToLower(item.Container), ParentID: parent, Overview: item.Plot, OfficialRating: item.Rating,
		DateCreated: item.Added.UTC().Format(time.RFC3339Nano), UserData: options.UserData, ProviderIDs: ProviderIDs(item.ProviderIDs),
		LocationType: "FileSystem", HasSubtitles: len(item.Subtitles) > 0, MediaSourceCount: 1,
	}
	if item.Artwork != "" {
		record.ImageTags = &ImageTags{Primary: id}
	}
	if year, err := strconv.Atoi(item.Year); item.Year != "" && err == nil {
		record.ProductionYear = &year
	}
	if item.Genres != "" {
		record.Genres = strings.Split(item.Genres, " · ")
	}
	if item.Show != "" {
		showID, showName := ShowID(item.Show), item.ShowTitle
		if showName == "" {
			showName = item.Show
		}
		record.EpisodeRecord = &EpisodeRecord{SeriesID: ID(showID), SeriesName: showName, SeasonID: SeasonID(showID, item.Season), ParentIndexNumber: item.Season, IndexNumber: item.Episode}
	}
	return record
}

func SeriesDTO(show library.Show) SeriesRecord {
	id := ID(show.ID)
	record := SeriesRecord{
		ID: id, Name: show.Title, SortName: show.Title, Type: "Series", MediaType: "Video", ParentID: ShowsID, IsFolder: true,
		ChildCount: len(show.Episodes), ProviderIDs: ShowProviderIDs(show.Episodes), LocationType: "FileSystem", UserData: FolderUserData{ItemID: id, Key: id},
	}
	if show.ArtworkID != "" {
		record.ImageTags = &ImageTags{Primary: show.ArtworkID}
	}
	return record
}

func SeasonDTO(show library.Show, season int) SeasonRecord {
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
	record := SeasonRecord{
		ID: id, Name: "Season " + strconv.Itoa(season), Type: "Season", MediaType: "Video", SeriesID: ID(show.ID), SeriesName: show.Title,
		ParentID: ID(show.ID), IndexNumber: season, IsFolder: true, ChildCount: count, UserData: FolderUserData{ItemID: id, Key: id},
	}
	if art != "" {
		record.ImageTags = &ImageTags{Primary: show.ArtworkID}
	}
	return record
}
