package catalog

import (
	"errors"
	"html/template"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	maximumDetailIDBytes = 512
	playerAlbumHTML      = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Title}} · Kinosail Player</title><script src="/static/theme.js?v=cinema-1"></script><link rel="stylesheet" href="/static/app.css?v=cinema-1"></head><body class="detail-page"><main class="detail-shell"><a class="back" href="/?view=music">{{icon "back"}} Music</a><section class="media-hero">{{if .ArtworkID}}<img class="hero-poster square" src="/art/{{.ArtworkID}}" alt="" loading="eager">{{else}}<div class="hero-poster square art">{{icon "music"}}</div>{{end}}<div><span class="eyebrow">Album</span><h1>{{.Title}}</h1>{{if .Artist}}<p class="meta-line">{{.Artist}}</p>{{end}}<p>{{len .Tracks}} tracks</p>{{with .First}}<a class="button hero-action" href="/watch/{{.ID}}">Play album</a>{{end}}</div></section><section class="detail-section"><header><span class="eyebrow">Track list</span><h2>{{.Title}}</h2></header><div class="episode-list track-list">{{range .Tracks}}<a class="episode" href="/watch/{{.ID}}"><span class="play">{{if .Track}}{{.Track}}{{else}}{{icon "play"}}{{end}}</span><span><strong>{{.Title}}</strong>{{if .Artist}}<small>{{.Artist}}</small>{{end}}</span><small>{{.Container}}</small></a>{{end}}</div></section></main></body></html>`
	playerBookHTML       = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Title}} · Kinosail Player</title><script src="/static/theme.js?v=cinema-1"></script><link rel="stylesheet" href="/static/app.css?v=cinema-1"></head><body class="detail-page"><main class="detail-shell"><a class="back" href="/?view=books">{{icon "back"}} Books</a><section class="media-hero">{{if .Artwork}}<img class="hero-poster" src="/art/{{.ID}}" alt="" loading="eager">{{else}}<div class="hero-poster art">{{icon "book"}}</div>{{end}}<div><span class="eyebrow">Book</span><h1>{{.Title}}</h1><p class="meta-line">{{.Container}}{{if .Year}} · {{.Year}}{{end}}</p>{{if .Plot}}<p>{{.Plot}}</p>{{end}}<div class="hero-actions"><a class="button" href="/read/{{.ID}}">Read now</a>{{if .CanDownload}}<a class="mode" href="/download/{{.ID}}">Download {{.Container}}</a>{{end}}<form action="/list/{{.ID}}" method="post"><button class="quiet" name="listed" value="{{if .Listed}}false{{else}}true{{end}}">{{if .Listed}}Remove from{{else}}Add to{{end}} My List</button></form></div></div></section>{{if or .Playlists .Collections}}<details class="curation-menu"><summary>Add to playlist or Collection</summary><div class="curation-options">{{if .Playlists}}<section><h2>Playlists</h2>{{range .Playlists}}<form action="/playlist/{{.Name}}/{{$.ID}}" method="post"><button name="included" value="{{if .Included}}false{{else}}true{{end}}" aria-label="{{if .Included}}Remove from{{else}}Add to{{end}} playlist · {{.Name}}"><span>{{.Name}}</span><small>{{if .Included}}Added{{else}}Add{{end}}</small></button></form>{{end}}</section>{{end}}{{if and .Owner .Collections}}<section><h2>Collections</h2>{{range .Collections}}<form action="/collection/{{.Name}}/{{$.ID}}" method="post"><button name="included" value="{{if .Included}}false{{else}}true{{end}}" aria-label="{{if .Included}}Remove from{{else}}Add to{{end}} Collection · {{.Name}}"><span>{{.Name}}</span><small>{{if .Included}}Added{{else}}Add{{end}}</small></button></form>{{end}}</section>{{end}}</div></details>{{end}}</main></body></html>`
)

// ErrInvalidDetailPresentation reports unsafe or incomplete product branding.
var ErrInvalidDetailPresentation = errors.New("invalid detail presentation")

// DetailPresentation declares the product-specific values used by shared media details.
type DetailPresentation struct {
	Product, ThemeVersion, StyleVersion string
}

// DetailSources contains Player-canonical album and book template sources.
type DetailSources struct {
	Album, Book string
}

// NewDetailSources applies validated product branding to Player's canonical markup.
func NewDetailSources(presentation DetailPresentation) (DetailSources, error) {
	if !validDetailPresentation(presentation) {
		return DetailSources{}, ErrInvalidDetailPresentation
	}
	replacer := strings.NewReplacer(
		"Kinosail Player", template.HTMLEscapeString(presentation.Product),
		"theme.js?v=cinema-1", "theme.js?v="+presentation.ThemeVersion,
		"app.css?v=cinema-1", "app.css?v="+presentation.StyleVersion,
	)
	return DetailSources{replacer.Replace(playerAlbumHTML), replacer.Replace(playerBookHTML)}, nil
}

// MustDetailSources returns validated detail sources or panics for invalid static configuration.
func MustDetailSources(presentation DetailPresentation) DetailSources {
	sources, err := NewDetailSources(presentation)
	if err != nil {
		panic(err)
	}
	return sources
}

func validDetailPresentation(presentation DetailPresentation) bool {
	if presentation.Product == "" || len(presentation.Product) > 64 || !utf8.ValidString(presentation.Product) || strings.TrimSpace(presentation.Product) != presentation.Product {
		return false
	}
	for _, character := range presentation.Product {
		if unicode.IsControl(character) {
			return false
		}
	}
	return validAssetVersion(presentation.ThemeVersion) && validAssetVersion(presentation.StyleVersion)
}

func validAssetVersion(version string) bool {
	if version == "" || len(version) > 32 {
		return false
	}
	for _, character := range version {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '.' && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

// DetailView renders one localized detail page.
type DetailView interface {
	Execute(http.ResponseWriter, *http.Request, any) error
}

// DetailIndex supplies Viewer-filtered Library state.
type DetailIndex interface {
	VisibleLibrary(*http.Request) []library.Item
	VisibleItem(*http.Request, string) (library.Item, bool)
}

// DetailLists supplies the app-owned curation state used by book pages.
type DetailLists interface {
	EditablePlaylistNames(*http.Request) []string
	Playlist(*http.Request, string, []library.Item) []library.Item
	CollectionNames([]library.Item) []string
	Collection(string, []library.Item) []library.Item
	Has(*http.Request, string) bool
}

// DetailViewer is the access policy used by a book page.
type DetailViewer struct {
	Owner, Downloads bool
}

// DetailHTTP owns Player-canonical album and book projection and response behavior.
type DetailHTTP struct {
	album, book DetailView
	viewer      func(*http.Request) DetailViewer
	failure     func(http.ResponseWriter, *http.Request, string, int)
	notFound    func(http.ResponseWriter, *http.Request)
}

// NewDetailHTTP binds app localization and state adapters to shared detail handlers.
func NewDetailHTTP(album, book DetailView, viewer func(*http.Request) DetailViewer, failure func(http.ResponseWriter, *http.Request, string, int), notFound func(http.ResponseWriter, *http.Request)) DetailHTTP {
	return DetailHTTP{album: album, book: book, viewer: viewer, failure: failure, notFound: notFound}
}

// Album returns the shared Player album handler.
func (details DetailHTTP) Album(index DetailIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		if !validDetailID(id) {
			details.notFound(writer, request)
			return
		}
		_, albums := library.OrganizeMusic(index.VisibleLibrary(request))
		for _, album := range albums {
			if album.ID == id {
				data := AlbumPage{Album: album}
				if len(album.Tracks) != 0 {
					data.First = &album.Tracks[0]
				}
				details.render(writer, request, details.album, data)
				return
			}
		}
		details.notFound(writer, request)
	}
}

// Book returns the shared Player book handler.
func (details DetailHTTP) Book(index DetailIndex, lists DetailLists) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		if !validDetailID(id) {
			details.notFound(writer, request)
			return
		}
		item, found := index.VisibleItem(request, id)
		if !found || item.Kind != "book" {
			details.notFound(writer, request)
			return
		}
		viewer := details.viewer(request)
		collections, playlists := make([]DetailOption, 0), make([]DetailOption, 0)
		items := index.VisibleLibrary(request)
		for _, name := range lists.EditablePlaylistNames(request) {
			playlists = append(playlists, DetailOption{name, len(lists.Playlist(request, name, []library.Item{item})) == 1})
		}
		for _, name := range lists.CollectionNames(items) {
			collections = append(collections, DetailOption{name, len(lists.Collection(name, []library.Item{item})) == 1})
		}
		data := BookPage{
			ID: item.ID, Title: item.Title, Plot: item.Plot, Container: item.Container, Year: item.Year,
			Artwork: item.Artwork != "", CanDownload: viewer.Owner || viewer.Downloads, Owner: viewer.Owner,
			Listed: lists.Has(request, item.ID), Playlists: playlists, Collections: collections,
		}
		details.render(writer, request, details.book, data)
	}
}

// AlbumPage is the canonical album template projection.
type AlbumPage struct {
	library.Album
	First *library.Item
}

// DetailOption is one book curation choice.
type DetailOption struct {
	Name     string
	Included bool
}

// BookPage is the canonical book template projection.
type BookPage struct {
	ID, Title, Plot, Container, Year string
	Artwork, CanDownload, Owner      bool
	Listed                           bool
	Playlists, Collections           []DetailOption
}

func (details DetailHTTP) render(writer http.ResponseWriter, request *http.Request, view DetailView, data any) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := view.Execute(writer, request, data); err != nil {
		details.failure(writer, request, err.Error(), http.StatusInternalServerError)
	}
}

func validDetailID(id string) bool {
	return id != "" && len(id) <= maximumDetailIDBytes && utf8.ValidString(id)
}
