package server

import "net/http"

func changeLibrary(settings *settingsStore, index *libraryIndex, change func(string) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := change(request.FormValue("path")); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		if err := index.UpdateRoots(request.Context(), settings.roots()); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}
