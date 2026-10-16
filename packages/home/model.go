package home

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/navigation"
)

type Shell struct {
	ServerName                        string
	Owner                             bool
	ViewerName, ViewerID              string
	CanLogout                         bool
	NavigationPrimary, NavigationMore []navigation.Link
	UpdateAvailable                   bool
}

func NewShell(serverName string, owner bool, viewerName, viewerID string, primary, more []navigation.Link, updateAvailable bool) Shell {
	return Shell{ServerName: serverName, Owner: owner, ViewerName: viewerName, ViewerID: viewerID, CanLogout: viewerID != "local-owner", NavigationPrimary: primary, NavigationMore: more, UpdateAvailable: updateAvailable}
}

type CatalogProjection[Show, Playlist, Collection any] struct {
	Shows                  []Show
	PlaylistCards          []Playlist
	CustomCollectionCards  []Collection
	DefaultCollectionCards []Collection
}

type PersonalProjection[Resume any] struct {
	Continue        []Resume
	List            []library.Item
	Played          []library.Item
	Unwatched       []library.Item
	CollectionCount int
	PlaylistCount   int
}

type Source[Show, Resume, Playlist, Collection any] interface {
	Browse(*http.Request) (catalog.Result, error)
	Shell(*http.Request, string) Shell
	Catalog(*http.Request, []library.Show, catalog.Result, bool) CatalogProjection[Show, Playlist, Collection]
	Personal(*http.Request, []library.Item) PersonalProjection[Resume]
	HasArtwork(library.Item) bool
	Library(*http.Request) *library.View
}

type View interface {
	Execute(http.ResponseWriter, *http.Request, any) error
	ExecuteTemplate(http.ResponseWriter, *http.Request, string, any) error
}

type Page[Show, Resume, Playlist, Collection any] struct {
	ServerName                        string
	Owner                             bool
	ViewerName, ViewerID              string
	CanLogout                         bool
	Items                             []library.Item
	Music                             []library.Item
	Audiobooks                        []library.Item
	Albums                            []library.Album
	Books                             []library.Item
	Photos                            []library.Item
	Shows                             []Show
	Continue                          []Resume
	List                              []library.Item
	Recent                            []ShelfItem
	MovieGenres                       []MovieGenre
	HomeShelves                       []HomeShelf
	Played                            []ShelfItem
	Query                             string
	View                              string
	Sort                              string
	PlaylistCards                     []Playlist
	CustomCollectionCards             []Collection
	DefaultCollectionCards            []Collection
	Destinations                      []DestinationGroup
	TMDB                              bool
	Total                             int
	Previous                          string
	Next                              string
	Letter                            string
	Letters                           []catalog.Letter
	SearchClear                       string
	NavigationPrimary, NavigationMore []navigation.Link
	HomeArtwork, HomeShowTitle        map[string]string
	UpdateAvailable                   bool
	CommandMenu                       bool
}
