package catalogapi

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// MediaGroup is the Player-canonical Show or Album summary.
type MediaGroup struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Artwork  string              `json:"artwork,omitempty"`
	Backdrop string              `json:"backdrop,omitempty"`
	Artist   string              `json:"artist,omitempty"`
	Play     *catalog.PlayAction `json:"play,omitempty"`
}

func (handlers MediaHandlers) registerBrowse(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/shows", handlers.shows)
	mux.HandleFunc("GET /api/v1/shows/{id}", handlers.show)
	mux.HandleFunc("GET /api/v1/albums", handlers.albums)
	mux.HandleFunc("GET /api/v1/albums/{id}", handlers.album)
}

func (handlers MediaHandlers) shows(writer http.ResponseWriter, request *http.Request) {
	_, shows := library.Organize(handlers.Index.VisibleLibrary(request))
	result := make([]MediaGroup, 0, len(shows))
	for _, show := range shows {
		play := catalog.ShowPlay(show, func(id string) catalog.PlaybackState { return handlers.Progress.Get(request, id) })
		result = append(result, MediaGroup{ID: show.ID, Title: show.Title, Artwork: ArtworkURL(show.ArtworkID), Backdrop: BackdropURL(ShowBackdropID(show), show.Backdrop != "" || show.Artwork != ""), Play: play})
	}
	apiAction{body: map[string]any{"shows": result}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) show(writer http.ResponseWriter, request *http.Request) {
	_, shows := library.Organize(handlers.Index.VisibleLibrary(request))
	for _, show := range shows {
		if show.ID == request.PathValue("id") {
			play := catalog.ShowPlay(show, func(id string) catalog.PlaybackState { return handlers.Progress.Get(request, id) })
			body := map[string]any{"id": show.ID, "title": show.Title, "backdrop": BackdropURL(ShowBackdropID(show), show.Backdrop != "" || show.Artwork != ""), "play": play, "episodes": handlers.Progress.ClientItems(request, show.Episodes), "cast": ShowCast(show), "year": show.Year, "plot": show.Plot, "genres": show.Genres, "studio": show.Studio}
			apiAction{body: body, status: http.StatusOK}.serve(writer)
			return
		}
	}
	apiAction{notFound: true}.serve(writer)
}

func (handlers MediaHandlers) albums(writer http.ResponseWriter, request *http.Request) {
	_, albums := library.OrganizeMusic(handlers.Index.VisibleLibrary(request))
	result := make([]MediaGroup, 0, len(albums))
	for _, album := range albums {
		result = append(result, MediaGroup{ID: album.ID, Title: album.Title, Artwork: ArtworkURL(album.ArtworkID), Artist: album.Artist})
	}
	apiAction{body: map[string]any{"albums": result}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) album(writer http.ResponseWriter, request *http.Request) {
	_, albums := library.OrganizeMusic(handlers.Index.VisibleLibrary(request))
	for _, album := range albums {
		if album.ID == request.PathValue("id") {
			body := map[string]any{"id": album.ID, "title": album.Title, "artist": album.Artist, "tracks": handlers.Progress.ClientItems(request, album.Tracks)}
			apiAction{body: body, status: http.StatusOK}.serve(writer)
			return
		}
	}
	apiAction{notFound: true}.serve(writer)
}

// ArtworkURL builds one canonical artwork route.
func ArtworkURL(id string) string {
	if id == "" {
		return ""
	}
	return "/art/" + id
}

// BackdropURL builds one available canonical backdrop route.
func BackdropURL(id string, available bool) string {
	if id == "" || !available {
		return ""
	}
	return "/backdrop/" + id
}

// ShowBackdropID selects the best Show-level backdrop route identifier.
func ShowBackdropID(show library.Show) string {
	if len(show.Episodes) != 0 && show.Backdrop != "" {
		return show.Episodes[0].ID
	}
	if show.ArtworkID != "" {
		return show.ArtworkID
	}
	return ""
}
