package home

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// InfiniteLibraryHeader requests only the bounded Library result fragment.
const InfiniteLibraryHeader = "X-Kinosail-Library-Page"

// NewHandler binds the shared projection to protected app state and templates.
func NewHandler[Show, Resume, Playlist, Collection any](source Source[Show, Resume, Playlist, Collection], view View, tmdb bool, failure func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Add("Vary", InfiniteLibraryHeader)
		fragment, err := FragmentRequest(request)
		if err != nil {
			failure(writer, request, "invalid library page request", http.StatusBadRequest)
			return
		}
		page, err := source.Browse(request)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, catalog.ErrInvalidBrowse) {
				status = http.StatusBadRequest
			}
			failure(writer, request, err.Error(), status)
			return
		}
		movies, organizedShows := library.Organize(page.Items)
		music, albums := library.OrganizeMusic(page.Items)
		shell := source.Shell(request, page.View)
		cards := source.Catalog(request, organizedShows, page, fragment)
		data := Page[Show, Resume, Playlist, Collection]{
			ServerName: shell.ServerName, Owner: shell.Owner, ViewerName: shell.ViewerName, ViewerID: shell.ViewerID, CanLogout: shell.CanLogout,
			Items: movies, Music: music, Audiobooks: Media(page.Items, "audiobook"), Albums: albums, Books: Media(page.Items, "book"), Photos: Media(page.Items, "photo"), Shows: cards.Shows,
			Query: page.Query, View: page.View, Sort: page.Sort, PlaylistCards: cards.PlaylistCards, CustomCollectionCards: cards.CustomCollectionCards, DefaultCollectionCards: cards.DefaultCollectionCards,
			TMDB: tmdb, Total: page.Total, Previous: page.PreviousURL(), Next: page.NextURL(), Letter: page.Letter, Letters: page.Letters, SearchClear: ClearSearchURL(request),
			NavigationPrimary: shell.NavigationPrimary, NavigationMore: shell.NavigationMore, UpdateAvailable: shell.UpdateAvailable, CommandMenu: true,
		}
		if !fragment && page.Query == "" && page.View == "all" && page.Offset == 0 {
			populateLanding(request, source, page.AllItems(), &data)
		}
		if renderErr := render(view, writer, request, data, fragment); renderErr != nil {
			slog.Error("render home", "error", renderErr)
		}
	}
}

func populateLanding[Show, Resume, Playlist, Collection any](request *http.Request, source Source[Show, Resume, Playlist, Collection], items []library.Item, page *Page[Show, Resume, Playlist, Collection]) {
	page.HomeArtwork, page.HomeShowTitle = MediaPresentation(items, source.HasArtwork)
	personal := source.Personal(request, items)
	page.Continue, page.List = personal.Continue, personal.List
	page.Played = RecentlyPlayed(personal.Played, page.HomeArtwork, page.HomeShowTitle)
	page.Recent = RecentlyAdded(items, page.HomeArtwork, page.HomeShowTitle)
	page.MovieGenres = MovieGenres(items, page.HomeArtwork)
	page.HomeShelves = HomeShelves(items, personal.Unwatched, page.HomeArtwork, page.HomeShowTitle, page.MovieGenres)
	page.Destinations = Destinations(items, len(personal.List), personal.CollectionCount, personal.PlaylistCount)
}

func render(view View, writer http.ResponseWriter, request *http.Request, data any, fragment bool) error {
	if fragment {
		return view.ExecuteTemplate(writer, request, "libraryResults", data)
	}
	return view.Execute(writer, request, data)
}
