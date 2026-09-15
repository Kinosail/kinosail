package library

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	episodeName = regexp.MustCompile(`(?i)^(.*?)\s*S(\d{1,2})E(\d{1,3})\s*(.*)$`)
	movieName   = regexp.MustCompile(`^(.+?)\s*[\[(]((?:19|20)\d{2})[\])](?:\s+(?:\{[^}]*\}|\[[^]]*\]).*)?$`)
	imdbName    = regexp.MustCompile(`(?i)\{imdb\s+(tt[0-9]{7,9})\}`)
)

// Item is playable Library Content discovered on a Kinosail Server.
type Item struct {
	ID              string
	Library         string
	LibraryType     string
	Kind            string
	Title           string
	SortTitle       string
	Year            string
	Plot            string
	Rating          string
	Tagline         string
	Genres          string
	Director        string
	Studio          string
	Collection      string
	Artist          string
	AlbumArtist     string
	Album           string
	Disc            int
	Track           int
	Lyrics          string
	Folder          string
	DateTaken       time.Time
	Container       string
	Size            int64
	Path            string
	Artwork         string
	Backdrop        string
	Logo            string
	ShowTitle       string
	ShowYear        string
	ShowPlot        string
	ShowGenres      string
	ShowStudio      string
	ShowArtwork     string
	ShowBackdrop    string
	ShowLogo        string
	SeasonArtwork   string
	ShowProviderIDs map[string]string
	Subtitle        string
	Subtitles       []string
	Show            string
	Season          int
	Episode         int
	Added           time.Time
	Cast            []Person
	ShowCast        []Person
	LocalTitle      bool
	ProviderIDs     map[string]string
}

// Person is a credited member of an item's cast.
type Person struct {
	Name  string
	Role  string
	Image string
}

// Show groups discovered episodes for browsing.
type Show struct {
	ID, Title, Year, Plot, Genres, Studio string
	Artwork, Backdrop, Logo               string
	ArtworkID                             string
	Cast                                  []Person
	Seasons                               []Season
	Episodes                              []Item
}

// Season groups discovered episodes and their season-specific artwork.
type Season struct {
	Number  int
	Artwork string
}

// Album groups locally described music tracks.
type Album struct {
	ID        string
	Title     string
	Artist    string
	ArtworkID string
	Tracks    []Item
}

func scanItem(root, namespace, path string, entry fs.DirEntry, kind string, catalog *scanCatalog) (Item, error) { //nolint:cyclop,funlen,gocognit // One scanned path is normalized into one complete library item.
	return scanItemWithRelative(root, namespace, path, entry, kind, catalog, filepath.Rel)
}

func scanItemWithRelative(root, namespace, path string, entry fs.DirEntry, kind string, catalog *scanCatalog, relativePath func(string, string) (string, error)) (Item, error) { //nolint:cyclop,funlen,gocognit // One scanned path is normalized into one complete library item.
	relative, err := relativePath(root, path)
	if err != nil {
		return Item{}, err
	}
	if (kind == "audio" || (kind == "video" && strings.EqualFold(filepath.Ext(path), ".mp4"))) && audiobookPath(namespace, relative) {
		kind = "audiobook"
	}
	info, err := entry.Info()
	if err != nil {
		return Item{}, err
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(filepath.Join(namespace, relative))))
	title := clean(strings.TrimSuffix(entry.Name(), filepath.Ext(path)))
	year := ""
	var providerIDs map[string]string
	show, showTitle, showYear, season, episodeNumber := "", "", "", 0, 0
	var showProviderIDs map[string]string
	if kind == "video" {
		show, season, episodeNumber, title = episode(relative, title)
		if show == "" {
			title, year, providerIDs = movie(title)
		} else {
			showTitle, showYear, showProviderIDs = movie(show)
		}
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	metadata := readMetadata(base + ".nfo")
	if metadata.Title != "" {
		title = metadata.Title
		if show != "" {
			title = fmt.Sprintf("S%02dE%02d · %s", season, episodeNumber, title)
		}
	}
	if metadata.Year != "" {
		year = metadata.Year
	}
	for provider, id := range metadata.providerIDs() {
		if providerIDs == nil {
			providerIDs = make(map[string]string)
		}
		providerIDs[provider] = id
	}
	directory := filepath.Dir(path)
	artwork := catalog.first(base+".jpg", base+".jpeg", base+".png", base+".webp", filepath.Join(directory, "poster.jpg"))
	backdrop := catalog.firstArtwork(base+"-fanart", base+"-backdrop", filepath.Join(directory, "fanart"), filepath.Join(directory, "backdrop"))
	logo := catalog.firstArtwork(base+"-clearlogo", base+"-logo", filepath.Join(directory, "clearlogo"), filepath.Join(directory, "logo"))
	showArtwork, showBackdrop, showLogo, seasonArtwork := "", "", "", ""
	if show != "" {
		showDirectory := filepath.Join(root, strings.Split(filepath.ToSlash(relative), "/")[0])
		artwork = catalog.firstArtwork(base+"-thumb", base)
		showArtwork = catalog.firstArtwork(filepath.Join(showDirectory, "poster"), filepath.Join(showDirectory, "folder"))
		showBackdrop = catalog.firstArtwork(filepath.Join(showDirectory, "fanart"), filepath.Join(showDirectory, "backdrop"))
		showLogo = catalog.firstArtwork(filepath.Join(showDirectory, "clearlogo"), filepath.Join(showDirectory, "logo"))
		seasonArtwork = catalog.firstArtwork(filepath.Join(directory, "poster"), filepath.Join(directory, "folder"))
		backdrop, logo = "", ""
	}
	if kind == "photo" {
		artwork = path
	}
	rating := metadata.MPAA
	if rating == "" {
		rating = metadata.ContentRating
	}
	subtitles := catalog.sidecarSubtitles(base)
	primary := ""
	if len(subtitles) > 0 {
		primary = subtitles[0]
	}
	return Item{ID: hex.EncodeToString(sum[:8]), Library: namespace, Kind: kind, Title: title, SortTitle: metadata.SortTitle, Year: year, Plot: metadata.Plot, Rating: rating, Tagline: metadata.Tagline, Genres: strings.Join(metadata.Genres, " · "), Director: metadata.Director, Studio: metadata.Studio, Artist: metadata.Artist, AlbumArtist: metadata.AlbumArtist, Album: metadata.Album, Disc: metadata.Disc, Track: metadata.Track, Lyrics: catalog.first(base+".lrc", base+".txt"), Folder: filepath.ToSlash(filepath.Dir(relative)), DateTaken: info.ModTime(), Container: strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), ".")), Size: info.Size(), Path: path, Artwork: artwork, Backdrop: backdrop, Logo: logo, ShowTitle: showTitle, ShowYear: showYear, ShowArtwork: showArtwork, ShowBackdrop: showBackdrop, ShowLogo: showLogo, SeasonArtwork: seasonArtwork, ShowProviderIDs: showProviderIDs, Subtitle: primary, Subtitles: subtitles, Show: show, Season: season, Episode: episodeNumber, Added: info.ModTime(), LocalTitle: metadata.Title != "", ProviderIDs: providerIDs}, nil
}

func (catalog *scanCatalog) firstArtwork(bases ...string) string {
	for _, base := range bases {
		if path := catalog.first(base+".jpg", base+".jpeg", base+".png", base+".webp"); path != "" {
			return path
		}
	}
	return ""
}

func (catalog *scanCatalog) sidecarSubtitles(base string) []string {
	paths := make([]string, 0, len(catalog.subtitles[base]))
	if exact := catalog.first(base+".vtt", base+".srt"); exact != "" {
		paths = append(paths, exact)
	}
	for _, path := range catalog.subtitles[base] {
		if path != base+".vtt" && path != base+".srt" {
			paths = append(paths, path)
		}
	}
	return paths
}

func episode(relative, title string) (string, int, int, string) {
	match := episodeName.FindStringSubmatch(title)
	if match == nil {
		return "", 0, 0, title
	}
	season, _ := strconv.Atoi(match[2])
	number, _ := strconv.Atoi(match[3])
	show := clean(match[1])
	if parts := strings.Split(filepath.ToSlash(relative), "/"); len(parts) > 1 {
		show = clean(parts[0])
	}
	episodeTitle := "S" + match[2] + "E" + match[3]
	if suffix := clean(match[4]); suffix != "" {
		episodeTitle += " · " + suffix
	}
	return show, season, number, episodeTitle
}

func clean(value string) string {
	value = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func movie(title string) (string, string, map[string]string) {
	var ids map[string]string
	if matches := imdbName.FindAllStringSubmatch(title, 2); len(matches) == 1 {
		ids = map[string]string{"imdb": strings.ToLower(matches[0][1])}
	}
	if match := movieName.FindStringSubmatch(title); match != nil {
		return strings.TrimSpace(match[1]), match[2], ids
	}
	return title, "", ids
}

func mediaKind(extension string) string {
	switch strings.ToLower(extension) {
	case ".3g2", ".3gp", ".asf", ".av1", ".avi", ".divx", ".dvr-ms", ".flv", ".h264", ".h265", ".h266", ".hevc", ".m2ts", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".mts", ".mxf", ".ogm", ".ogv", ".rm", ".rmvb", ".ts", ".vob", ".vvc", ".webm", ".wmv", ".wtv":
		return "video"
	case ".aac", ".ac3", ".aif", ".aiff", ".aifc", ".alac", ".ape", ".caf", ".mpc", ".wv", ".dts", ".eac3", ".flac", ".m4a", ".mp3", ".oga", ".ogg", ".opus", ".tta", ".wav", ".weba", ".wma":
		return "audio"
	case ".m4b":
		return "audiobook"
	case ".avif", ".bmp", ".gif", ".jpeg", ".jpg", ".png", ".webp":
		return "photo"
	case ".cb7", ".cbt", ".cbz", ".epub", ".pdf":
		return "book"
	default:
		return ""
	}
}

func (catalog *scanCatalog) sidecarArtwork(path string) bool { //nolint:cyclop // Fixed artwork names and suffixes remain an explicit allowlist.
	base := strings.TrimSuffix(path, filepath.Ext(path))
	if catalog.hasMedia(base) {
		return true
	}
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if name == "poster" || name == "fanart" || name == "backdrop" || name == "thumb" || name == "folder" || name == "clearlogo" || name == "logo" {
		return true
	}
	for _, suffix := range []string{"-thumb", "-fanart", "-backdrop", "-clearlogo", "-logo"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func audiobookPath(namespace, relative string) bool {
	for part := range strings.SplitSeq(strings.ToLower(filepath.ToSlash(namespace+"/"+relative)), "/") {
		if part == "audiobook" || part == "audiobooks" {
			return true
		}
	}
	return false
}
