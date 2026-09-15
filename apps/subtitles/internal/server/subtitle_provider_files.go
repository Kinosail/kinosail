package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func subtitleSidecarPath(item library.Item, language string) string {
	return strings.TrimSuffix(item.Path, filepath.Ext(item.Path)) + "." + language + ".srt"
}

func (provider *subtitleProvider) path(id, language string) string {
	return filepath.Join(provider.cache, "subtitles", id+"."+language+".srt")
}

func (provider *subtitleProvider) cached(id, language string) string {
	path := provider.path(id, language)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

func (provider *subtitleProvider) serve(writer http.ResponseWriter, request *http.Request) {
	item, found := visibleItem(request, provider.index, request.PathValue("id"))
	language := request.PathValue("language")
	path := provider.cached(item.ID, language)
	if !found || !validLanguage(language) || path == "" {
		localizedNotFound(writer, request)
		return
	}
	writeSubtitle(writer, request, path)
}

func validLanguage(language string) bool {
	_, ok := subtitlelanguage.NormalizeTag(language)
	return ok
}
