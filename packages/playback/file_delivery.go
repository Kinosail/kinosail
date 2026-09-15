package playback

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type LibraryFileDependencies struct {
	Lookup      func(*http.Request, string) (library.Item, bool)
	Select      func(library.Item) string
	Safe        func(string) bool
	NotFound    func(http.ResponseWriter, *http.Request)
	ContentType string
}

type LibraryDownloadDependencies struct {
	Allowed  func(*http.Request) bool
	Lookup   func(*http.Request, string) (library.Item, bool)
	NotFound func(http.ResponseWriter, *http.Request)
	Error    func(http.ResponseWriter, *http.Request, string, int)
}

// LibraryFileHandler serves one safe file selected from a visible Library item.
func LibraryFileHandler(dependencies LibraryFileDependencies) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || !validLibraryFileDependencies(dependencies) {
			http.Error(writer, "file delivery is unavailable", http.StatusInternalServerError)
			return
		}
		item, found := dependencies.Lookup(request, request.PathValue("id"))
		if !found {
			dependencies.NotFound(writer, request)
			return
		}
		path := dependencies.Select(item)
		if path == "" || !dependencies.Safe(path) {
			dependencies.NotFound(writer, request)
			return
		}
		if dependencies.ContentType != "" {
			writer.Header().Set("Content-Type", dependencies.ContentType)
		} else if contentType := map[string]string{".m4a": "audio/mp4", ".m4b": "audio/mp4", ".flac": "audio/flac"}[strings.ToLower(filepath.Ext(path))]; contentType != "" {
			writer.Header().Set("Content-Type", contentType)
		}
		//nolint:gosec // G703: the app validates that path belongs to scanned Library Content.
		http.ServeFile(writer, request, path)
	}
}

func validLibraryFileDependencies(dependencies LibraryFileDependencies) bool {
	return dependencies.Lookup != nil && dependencies.Select != nil && dependencies.Safe != nil && dependencies.NotFound != nil && len(dependencies.ContentType) <= 256 && !strings.ContainsAny(dependencies.ContentType, "\r\n")
}

// LibraryPersonHandler serves one indexed cast image from a visible item.
func LibraryPersonHandler(lookup func(*http.Request, string) (library.Item, bool)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || lookup == nil {
			http.Error(writer, "file delivery is unavailable", http.StatusInternalServerError)
			return
		}
		item, found := lookup(request, request.PathValue("id"))
		if request.URL.RawQuery != "" {
			if request.URL.RawQuery != "scope=show" {
				http.NotFound(writer, request)
				return
			}
			item.Cast = item.ShowCast
		}
		person, err := strconv.Atoi(request.PathValue("person"))
		if !found || err != nil || person < 0 || person >= len(item.Cast) || item.Cast[person].Image == "" {
			http.NotFound(writer, request)
			return
		}
		//nolint:gosec // G703: the image path belongs to an indexed Library cast member.
		http.ServeFile(writer, request, item.Cast[person].Image)
	}
}

// LibraryDownloadHandler authorizes one visible item before serving it as an attachment.
func LibraryDownloadHandler(dependencies LibraryDownloadDependencies) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || dependencies.Allowed == nil || dependencies.Lookup == nil || dependencies.NotFound == nil || dependencies.Error == nil {
			http.Error(writer, "file delivery is unavailable", http.StatusInternalServerError)
			return
		}
		if !dependencies.Allowed(request) {
			dependencies.Error(writer, request, "downloads are not enabled for this Viewer Profile", http.StatusForbidden)
			return
		}
		item, found := dependencies.Lookup(request, request.PathValue("id"))
		if !found {
			dependencies.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filepath.Base(item.Path)))
		//nolint:gosec // G703: the media path belongs to a visible indexed Library item.
		http.ServeFile(writer, request, item.Path)
	}
}

func MediaPath(item library.Item) string { return item.Path }

func ArtworkPath(item library.Item) string {
	if item.ShowArtwork != "" {
		return item.ShowArtwork
	}
	return item.Artwork
}

func BackdropPath(item library.Item) string {
	if item.ShowBackdrop != "" {
		return item.ShowBackdrop
	}
	if item.Backdrop != "" {
		return item.Backdrop
	}
	return ArtworkPath(item)
}
