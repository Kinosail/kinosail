package library

import (
	"encoding/xml"
	"io"
	"os"
	"strings"
)

type localMetadata struct {
	Title         string     `xml:"title"`
	SortTitle     string     `xml:"sorttitle"`
	Year          string     `xml:"year"`
	Plot          string     `xml:"plot"`
	MPAA          string     `xml:"mpaa"`
	ContentRating string     `xml:"contentrating"`
	Tagline       string     `xml:"tagline"`
	Genres        []string   `xml:"genre"`
	Director      string     `xml:"director"`
	Studio        string     `xml:"studio"`
	Artist        string     `xml:"artist"`
	AlbumArtist   string     `xml:"albumartist"`
	Album         string     `xml:"album"`
	Disc          int        `xml:"disc"`
	Track         int        `xml:"track"`
	UniqueIDs     []uniqueID `xml:"uniqueid"`
}

type uniqueID struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

func (metadata localMetadata) providerIDs() map[string]string {
	ids := make(map[string]string)
	for _, id := range metadata.UniqueIDs {
		provider, value := strings.ToLower(strings.TrimSpace(id.Type)), strings.TrimSpace(id.Value)
		if (provider == "tmdb" || provider == "tvdb" || provider == "imdb") && value != "" {
			ids[provider] = value
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func readMetadata(path string) localMetadata {
	return readMetadataWith(path, os.Open)
}

func readMetadataWith(path string, open func(string) (*os.File, error)) localMetadata {
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return localMetadata{}
	}
	file, err := open(path) // #nosec G304 -- fixed sidecar beside scanned Library Content.
	if err != nil {
		return localMetadata{}
	}
	defer func() { _ = file.Close() }()
	var metadata localMetadata
	if xml.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&metadata) != nil {
		return localMetadata{}
	}
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.SortTitle = strings.TrimSpace(metadata.SortTitle)
	if len(metadata.SortTitle) > 512 {
		metadata.SortTitle = ""
	}
	metadata.Year = strings.TrimSpace(metadata.Year)
	metadata.Plot = strings.TrimSpace(metadata.Plot)
	metadata.MPAA = strings.TrimSpace(metadata.MPAA)
	metadata.ContentRating = strings.TrimSpace(metadata.ContentRating)
	metadata.Tagline = strings.TrimSpace(metadata.Tagline)
	metadata.Director = strings.TrimSpace(metadata.Director)
	metadata.Studio = strings.TrimSpace(metadata.Studio)
	metadata.Artist = strings.TrimSpace(metadata.Artist)
	metadata.AlbumArtist = strings.TrimSpace(metadata.AlbumArtist)
	metadata.Album = strings.TrimSpace(metadata.Album)
	for index := range metadata.Genres {
		metadata.Genres[index] = strings.TrimSpace(metadata.Genres[index])
	}
	return metadata
}
