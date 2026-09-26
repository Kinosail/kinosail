// Package home owns the Player-canonical Library home projection.
package home

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/navigation"
)

// Shell is app-owned identity, navigation, and update state.
type Shell struct {
	ServerName                        string
	Owner                             bool
	ViewerName, ViewerID              string
	CanLogout                         bool
	NavigationPrimary, NavigationMore []navigation.Link
	UpdateAvailable                   bool
}

// NewShell projects app-owned identity and navigation with canonical logout rules.
func NewShell(serverName string, owner bool, viewerName, viewerID string, primary, more []navigation.Link, updateAvailable bool) Shell {
	return Shell{ServerName: serverName, Owner: owner, ViewerName: viewerName, ViewerID: viewerID, CanLogout: viewerID != "local-owner", NavigationPrimary: primary, NavigationMore: more, UpdateAvailable: updateAvailable}
}

// CatalogProjection contains app-specific card types for one Library page.
type CatalogProjection[Show, Playlist, Collection any] struct {
	Shows                  []Show
	PlaylistCards          []Playlist
	CustomCollectionCards  []Collection
	DefaultCollectionCards []Collection
}

// PersonalProjection contains Viewer-specific home shelves and counts.
type PersonalProjection[Resume any] struct {
	Continue        []Resume
	List            []library.Item
	Played          []library.Item
	CollectionCount int
	PlaylistCount   int
}

// Source is the app adapter for protected state and app-specific card types.
type Source[Show, Resume, Playlist, Collection any] interface {
	Browse(*http.Request) (catalog.Result, error)
	Shell(*http.Request, string) Shell
	Catalog(*http.Request, []library.Show, catalog.Result, bool) CatalogProjection[Show, Playlist, Collection]
	Personal(*http.Request, []library.Item) PersonalProjection[Resume]
	HasArtwork(library.Item) bool
}

// View keeps app-owned templates and localization at the delivery edge.
type View interface {
	Execute(http.ResponseWriter, *http.Request, any) error
	ExecuteTemplate(http.ResponseWriter, *http.Request, string, any) error
}

// Page is the stable template projection shared by Player-derived apps.
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
