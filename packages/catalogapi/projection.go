package catalogapi

import (
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

type ItemAccess struct {
	Stream   bool
	Download bool
}

type ClientItem struct {
	ID        string                `json:"id"`
	Kind      string                `json:"kind"`
	Title     string                `json:"title"`
	SortTitle string                `json:"sortTitle,omitempty"`
	Year      string                `json:"year,omitempty"`
	Plot      string                `json:"plot,omitempty"`
	Rating    string                `json:"rating,omitempty"`
	Tagline   string                `json:"tagline,omitempty"`
	Genres    string                `json:"genres,omitempty"`
	Director  string                `json:"director,omitempty"`
	Studio    string                `json:"studio,omitempty"`
	Artist    string                `json:"artist,omitempty"`
	Album     string                `json:"album,omitempty"`
	Track     int                   `json:"track,omitempty"`
	ShowID    string                `json:"showId,omitempty"`
	Show      string                `json:"show,omitempty"`
	Season    int                   `json:"season,omitempty"`
	Episode   int                   `json:"episode,omitempty"`
	Stream    string                `json:"stream,omitempty"`
	Artwork   string                `json:"artwork,omitempty"`
	Backdrop  string                `json:"backdrop,omitempty"`
	Download  string                `json:"download,omitempty"`
	Subtitles int                   `json:"subtitles,omitempty"`
	Container string                `json:"container,omitempty"`
	Size      int64                 `json:"size,omitempty"`
	Added     time.Time             `json:"added,omitempty"`
	Cast      []ClientPerson        `json:"cast,omitempty"`
	Progress  catalog.PlaybackState `json:"progress"`
}

type ClientPerson struct {
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`
	Image string `json:"image,omitempty"`
}

func ProjectItem(item library.Item, progress catalog.PlaybackState, access ItemAccess) ClientItem {
	result := ClientItem{ID: item.ID, Kind: item.Kind, Title: item.Title, SortTitle: item.SortTitle, Year: item.Year, Plot: item.Plot, Rating: item.Rating, Tagline: item.Tagline, Genres: item.Genres, Director: item.Director, Studio: item.Studio, Artist: item.Artist, Album: item.Album, Track: item.Track, Show: item.Show, Season: item.Season, Episode: item.Episode, Subtitles: len(item.Subtitles), Container: item.Container, Size: item.Size, Added: item.Added, Cast: projectCast(item), Progress: progress}
	if item.Kind == "video" && item.Show != "" {
		result.ShowID = library.ShowID(item.Show)
		if item.ShowTitle != "" {
			result.Show = item.ShowTitle
		}
	}
	if access.Stream {
		result.Stream = "/media/" + item.ID
	}
	result.Artwork = projectedArtwork(item)
	if item.ShowBackdrop != "" || item.Backdrop != "" {
		result.Backdrop = "/backdrop/" + item.ID
	}
	if access.Download {
		result.Download = "/download/" + item.ID
	}
	return result
}

func projectedArtwork(item library.Item) string {
	if item.Artwork != "" {
		return ArtworkURLForItem(item)
	}
	if item.Kind == "video" && (item.Show != "" || item.Backdrop == "") {
		return "/episode-art/" + item.ID
	}
	return ""
}

func projectCast(item library.Item) []ClientPerson {
	if len(item.Cast) == 0 {
		item.Cast = item.ShowCast
	}
	cast := make([]ClientPerson, 0, len(item.Cast))
	for index, person := range item.Cast {
		image := ""
		if person.Image != "" {
			image = "/person/" + item.ID + "/" + strconv.Itoa(index)
		}
		cast = append(cast, ClientPerson{Name: person.Name, Role: person.Role, Image: image})
	}
	return cast
}

func ArtworkURLForItem(item library.Item) string {
	if item.Show != "" {
		return "/art/" + item.ID + "?variant=episode"
	}
	return "/art/" + item.ID
}

func ShowCast(show library.Show) []ClientPerson {
	for _, episode := range show.Episodes {
		if len(episode.ShowCast) == 0 {
			continue
		}
		episode.Cast = episode.ShowCast
		cast := projectCast(episode)
		for index := range cast {
			if cast[index].Image != "" {
				cast[index].Image += "?scope=show"
			}
		}
		return cast
	}
	return nil
}
