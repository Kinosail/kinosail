package catalogapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

const (
	PlaylistDocumentFormat = "kinosail.playlist"
	PlaylistExportQuery    = "kinosail"
	maximumPlaylistBytes   = 1 << 20
)

// PlaylistDocument is the portable Player playlist format.
type PlaylistDocument struct {
	Format  string   `json:"format,omitempty"`
	Version int      `json:"version,omitempty"`
	Name    string   `json:"name"`
	IDs     []string `json:"ids"`
}

// NewListHandlers binds portable playlist documents to the canonical web mutations.
func NewListHandlers(index MediaIndex, lists MediaLists, viewer func(*http.Request) string, failure func(http.ResponseWriter, *http.Request, string, int), notFound func(http.ResponseWriter, *http.Request)) catalog.ListHandlers {
	return catalog.NewListHandlers(index, lists, viewer, func(request *http.Request, raw string) (string, error) {
		document, err := DecodePlaylistDocument(raw)
		if err != nil {
			return "", err
		}
		return CreatePlaylistDocument(request.Context(), viewer(request), document, func(id string) (library.Item, bool) {
			return index.VisibleItem(request, id)
		}, lists.Create)
	}, failure, notFound)
}

// DecodePlaylistDocument strictly decodes one bounded playlist document.
func DecodePlaylistDocument(raw string) (PlaylistDocument, error) {
	if len(raw) == 0 || len(raw) > maximumPlaylistBytes {
		return PlaylistDocument{}, errors.New("playlist document is invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document PlaylistDocument
	if err := decoder.Decode(&document); err != nil {
		return PlaylistDocument{}, errors.New("playlist document is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return PlaylistDocument{}, errors.New("playlist document is invalid")
	}
	return document, nil
}

// ValidatePlaylist validates one named playlist and its stable item order.
func ValidatePlaylist(name string, ids []string) (string, error) {
	return catalog.ValidatePlaylist(name, ids)
}

// ValidatePlaylistDocument accepts legacy named playlists and version one exports.
func ValidatePlaylistDocument(document PlaylistDocument) (string, error) {
	if document.Format == "" && document.Version == 0 {
		return ValidatePlaylist(document.Name, document.IDs)
	}
	if document.Format != PlaylistDocumentFormat || document.Version != 1 {
		return "", errors.New("playlist document is invalid")
	}
	return ValidatePlaylist(document.Name, document.IDs)
}

// CreatePlaylistDocument validates every visible item before persistence.
func CreatePlaylistDocument(ctx context.Context, viewer string, document PlaylistDocument, visible func(string) (library.Item, bool), create func(context.Context, string, string, ...string) error) (string, error) {
	name, err := ValidatePlaylistDocument(document)
	if err != nil {
		return "", err
	}
	for _, id := range document.IDs {
		if _, found := visible(id); !found {
			return "", errors.New("playlist item is unavailable")
		}
	}
	if err := create(ctx, viewer, name, document.IDs...); err != nil {
		return "", err
	}
	return name, nil
}

// ExportPlaylistDocument builds one stable manual-playlist document.
func ExportPlaylistDocument(name string, selected map[string]bool, order []string, smart bool) (PlaylistDocument, error) {
	if selected == nil || smart {
		return PlaylistDocument{}, os.ErrNotExist
	}
	ids := make([]string, 0, len(selected))
	seen := make(map[string]bool, len(selected))
	for _, id := range order {
		if selected[id] && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	remaining := make([]string, 0, len(selected)-len(ids))
	for id, included := range selected {
		if included && !seen[id] {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	ids = append(ids, remaining...)
	return PlaylistDocument{Format: PlaylistDocumentFormat, Version: 1, Name: name, IDs: ids}, nil
}
