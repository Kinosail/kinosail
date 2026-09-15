package library

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Organize separates movies and grouped shows.
func Organize(items []Item) ([]Item, []Show) {
	movies := make([]Item, 0)
	grouped := make(map[string]*Show)
	keys, episodes := videoGroupKeys(items)
	for position, item := range items {
		if item.Kind != "video" {
			continue
		}
		if item.Show == "" {
			movies = append(movies, item)
			continue
		}
		key := keys[position]
		show := grouped[key]
		if show == nil {
			show = newShow(item, key, episodes[key])
			grouped[key] = show
		}
		show.Episodes = append(show.Episodes, item)
		mergeShow(show, item)
	}
	return movies, organizedShows(grouped)
}

func videoGroupKeys(items []Item) ([]string, map[string]int) {
	episodes := make(map[string]int)
	keys := make([]string, len(items))
	for position, item := range items {
		if item.Kind == "video" && item.Show != "" {
			keys[position] = strings.ToLower(item.Show)
			episodes[keys[position]]++
		}
	}
	return keys, episodes
}

// ShowID returns the same stable identity used to group a Show's episodes.
func ShowID(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name)))
	return hex.EncodeToString(sum[:8])
}

func newShow(item Item, key string, episodeCount int) *Show {
	id := ShowID(key)
	title := item.Show
	if item.ShowTitle != "" {
		title = item.ShowTitle
	}
	return &Show{ID: id, Title: title, Year: item.ShowYear, Plot: item.ShowPlot, Genres: item.ShowGenres, Studio: item.ShowStudio, Artwork: item.ShowArtwork, Backdrop: item.ShowBackdrop, Logo: item.ShowLogo, Cast: item.ShowCast, Episodes: make([]Item, 0, episodeCount)}
}

func mergeShow(show *Show, item Item) { //nolint:cyclop // Each condition fills one independent optional presentation field.
	if item.ShowTitle != "" {
		show.Title = item.ShowTitle
	}
	fill(&show.Year, item.ShowYear)
	fill(&show.Plot, item.ShowPlot)
	fill(&show.Genres, item.ShowGenres)
	fill(&show.Studio, item.ShowStudio)
	if len(show.Cast) == 0 {
		show.Cast = item.ShowCast
	}
	fill(&show.Artwork, item.ShowArtwork)
	fill(&show.Artwork, item.Artwork)
	fill(&show.Backdrop, item.ShowBackdrop)
	fill(&show.Logo, item.ShowLogo)
	if show.ArtworkID == "" && show.Artwork != "" {
		show.ArtworkID = item.ID
	}
}

func organizedShows(grouped map[string]*Show) []Show {
	shows := make([]Show, 0, len(grouped))
	for _, show := range grouped {
		organizeEpisodes(show)
		shows = append(shows, *show)
	}
	sort.Slice(shows, func(left, right int) bool { return shows[left].Title < shows[right].Title })
	return shows
}

func organizeEpisodes(show *Show) {
	sort.Slice(show.Episodes, func(left, right int) bool {
		return show.Episodes[left].Season*1000+show.Episodes[left].Episode < show.Episodes[right].Season*1000+show.Episodes[right].Episode
	})
	for _, episode := range show.Episodes {
		if len(show.Seasons) == 0 || show.Seasons[len(show.Seasons)-1].Number != episode.Season {
			show.Seasons = append(show.Seasons, Season{Number: episode.Season, Artwork: episode.SeasonArtwork})
		}
		season := &show.Seasons[len(show.Seasons)-1]
		if season.Artwork == "" {
			season.Artwork = episode.SeasonArtwork
		}
	}
}

func fill(target *string, value string) {
	if *target == "" {
		*target = value
	}
}

// OrganizeMusic separates loose tracks from albums.
func OrganizeMusic(items []Item) ([]Item, []Album) {
	loose := make([]Item, 0)
	grouped := make(map[string]*Album)
	for _, item := range items {
		if item.Kind != "audio" {
			continue
		}
		if item.Album == "" {
			loose = append(loose, item)
			continue
		}
		artist := item.AlbumArtist
		if artist == "" {
			artist = item.Artist
		}
		key := strings.ToLower(artist + "\x00" + item.Album)
		if grouped[key] == nil {
			sum := sha256.Sum256([]byte(key))
			grouped[key] = &Album{ID: hex.EncodeToString(sum[:8]), Title: item.Album, Artist: artist}
		}
		album := grouped[key]
		album.Tracks = append(album.Tracks, item)
		if album.ArtworkID == "" && item.Artwork != "" {
			album.ArtworkID = item.ID
		}
	}
	albums := make([]Album, 0, len(grouped))
	for _, album := range grouped {
		sort.SliceStable(album.Tracks, func(left, right int) bool {
			return album.Tracks[left].Disc*1000+album.Tracks[left].Track < album.Tracks[right].Disc*1000+album.Tracks[right].Track
		})
		albums = append(albums, *album)
	}
	sort.Slice(albums, func(left, right int) bool {
		return albums[left].Artist+albums[left].Title < albums[right].Artist+albums[right].Title
	})
	return loose, albums
}
