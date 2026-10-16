package catalogapi

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

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
	shows := handlers.Index.VisibleView(request).Shows()
	result := make([]MediaGroup, 0, len(shows))
	for _, show := range shows {
		play := catalog.ShowPlay(show, func(id string) catalog.PlaybackState { return handlers.Progress.Get(request, id) })
		result = append(result, MediaGroup{ID: show.ID, Title: show.Title, Artwork: ArtworkURL(show.ArtworkID), Backdrop: BackdropURL(ShowBackdropID(show), show.Backdrop != ""), Play: play})
	}
	apiAction{body: map[string]any{"shows": result}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) show(writer http.ResponseWriter, request *http.Request) {
	show, found := handlers.Index.VisibleView(request).Show(request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	play := catalog.ShowPlay(show, func(id string) catalog.PlaybackState { return handlers.Progress.Get(request, id) })
	body := map[string]any{"id": show.ID, "title": show.Title, "backdrop": BackdropURL(ShowBackdropID(show), show.Backdrop != ""), "play": play, "episodes": handlers.Progress.ClientItems(request, show.Episodes), "cast": ShowCast(show), "year": show.Year, "plot": show.Plot, "genres": show.Genres, "studio": show.Studio}
	apiAction{body: body, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) albums(writer http.ResponseWriter, request *http.Request) {
	albums := handlers.Index.VisibleView(request).Albums()
	result := make([]MediaGroup, 0, len(albums))
	for _, album := range albums {
		result = append(result, MediaGroup{ID: album.ID, Title: album.Title, Artwork: ArtworkURL(album.ArtworkID), Artist: album.Artist})
	}
	apiAction{body: map[string]any{"albums": result}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) album(writer http.ResponseWriter, request *http.Request) {
	album, found := handlers.Index.VisibleView(request).Album(request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	body := map[string]any{"id": album.ID, "title": album.Title, "artist": album.Artist, "tracks": handlers.Progress.ClientItems(request, album.Tracks)}
	apiAction{body: body, status: http.StatusOK}.serve(writer)
}

func ArtworkURL(id string) string {
	if id == "" {
		return ""
	}
	return "/art/" + id
}

func BackdropURL(id string, available bool) string {
	if id == "" || !available {
		return ""
	}
	return "/backdrop/" + id
}

func ShowBackdropID(show library.Show) string {
	if len(show.Episodes) != 0 && show.Backdrop != "" {
		return show.Episodes[0].ID
	}
	if show.ArtworkID != "" {
		return show.ArtworkID
	}
	return ""
}
