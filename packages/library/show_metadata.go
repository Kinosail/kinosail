package library

import (
	"io/fs"
	"path/filepath"
	"strings"
)

const showMetadataName = "tvshow.nfo"

func (directory *scanDir) showMetadata(path string, byName map[string]fs.DirEntry, previous *scanDir) (sidecarStamp, localMetadata) {
	if _, found := directory.regular[showMetadataName]; !found {
		return sidecarStamp{}, localMetadata{}
	}
	info, err := byName[showMetadataName].Info()
	if err != nil || !info.Mode().IsRegular() {
		return sidecarStamp{}, localMetadata{}
	}
	stamp := sidecarStamp{present: true, size: info.Size(), modified: info.ModTime().UnixNano()}
	if previous != nil && previous.showFile == stamp {
		return stamp, previous.show
	}
	return stamp, boundedShowMetadata(readMetadata(filepath.Join(path, showMetadataName)))
}

func boundedShowMetadata(metadata localMetadata) localMetadata {
	for _, field := range []struct {
		value *string
		limit int
	}{{&metadata.Title, 200}, {&metadata.Plot, 5000}, {&metadata.Studio, 200}, {&metadata.MPAA, 16}, {&metadata.ContentRating, 16}} {
		if len(*field.value) > field.limit {
			*field.value = ""
		}
	}
	if len(metadata.Year) != 4 || strings.Trim(metadata.Year, "0123456789") != "" {
		metadata.Year = ""
	}
	genres := make([]string, 0, len(metadata.Genres))
	for _, genre := range metadata.Genres {
		if genre != "" && len(genre) <= 64 && len(genres) < 16 {
			genres = append(genres, genre)
		}
	}
	metadata.Genres = genres
	return metadata
}

func (metadata localMetadata) applyShow(item *Item) {
	genres, ids := strings.Join(metadata.Genres, " · "), metadata.providerIDs()
	rating := metadata.MPAA
	if rating == "" {
		rating = metadata.ContentRating
	}
	if metadata.Title == "" && metadata.Year == "" && metadata.Plot == "" && metadata.Studio == "" && genres == "" && rating == "" && ids == nil {
		return
	}
	item.LocalShowMetadata = true
	if metadata.Title != "" {
		item.ShowTitle = metadata.Title
	}
	if metadata.Year != "" {
		item.ShowYear = metadata.Year
	}
	item.ShowPlot, item.ShowGenres, item.ShowStudio = metadata.Plot, genres, metadata.Studio
	if item.Rating == "" {
		item.Rating = rating
	}
	if ids != nil {
		for provider, id := range item.ShowProviderIDs {
			if _, found := ids[provider]; !found {
				ids[provider] = id
			}
		}
		item.ShowProviderIDs = ids
	}
}
