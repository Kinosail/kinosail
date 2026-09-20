package downloads

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

// Access supplies application authorization and visible Library items.
type Access interface {
	Profile(*http.Request) (string, bool)
	Item(*http.Request, string) (string, library.Item, bool)
}

type api struct {
	manager *Manager
	access  Access
}

// RegisterAPI registers Player's versioned offline-download contract.
func RegisterAPI(mux *http.ServeMux, manager *Manager, access Access) {
	handler := api{manager, access}
	mux.HandleFunc("POST /api/v1/items/{id}/downloads", handler.start)
	mux.HandleFunc("GET /api/v1/items/{id}/download-tracks", handler.tracks)
	mux.HandleFunc("GET /api/v1/downloads", handler.list)
	mux.HandleFunc("GET /api/v1/downloads/identity", handler.identity)
	mux.HandleFunc("GET /api/v1/downloads/{id}/manifest", handler.manifest)
	mux.HandleFunc("GET /api/v1/downloads/{id}", handler.get)
	mux.HandleFunc("GET /api/v1/downloads/{id}/file", handler.file)
	mux.HandleFunc("DELETE /api/v1/downloads/{id}", handler.remove)
}

func (handler api) start(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Quality string          `json:"quality"`
		Tracks  *TrackSelection `json:"tracks,omitempty"`
	}
	profileID, item, found := handler.access.Item(request, request.PathValue("id"))
	if !found {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	if !readJSON(writer, request, &input) {
		return
	}
	job, err := handler.manager.StartSelected(profileID, item, input.Quality, input.Tracks)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrQueueFull) {
			status = http.StatusServiceUnavailable
			writer.Header().Set("Retry-After", "15")
		}
		if errors.Is(err, ErrCapacity) {
			status = http.StatusConflict
		}
		writeError(writer, err, status)
		return
	}
	writeJSON(writer, job, http.StatusAccepted)
}

func (handler api) list(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	if !allowed {
		writeError(writer, errors.New("download access required"), http.StatusForbidden)
		return
	}
	writeJSON(writer, map[string]any{"downloads": visibleJobs(handler.access, request, profileID, handler.manager.List(profileID))}, http.StatusOK)
}

func (handler api) get(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	if !allowed {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	job, found := handler.manager.Get(profileID, request.PathValue("id"))
	if !found || !visibleJob(handler.access, request, profileID, job) {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	writeJSON(writer, job, http.StatusOK)
}

func (handler api) file(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	job, found := handler.manager.Get(profileID, request.PathValue("id"))
	if !allowed || !found || !visibleJob(handler.access, request, profileID, job) || job.State != "ready" || Serve(writer, request, job) != nil {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
	}
}

func (handler api) remove(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	if !allowed {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	if err := handler.manager.Remove(profileID, request.PathValue("id")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(writer, errors.New("not found"), http.StatusNotFound)
		} else {
			writeError(writer, errors.New("download could not be removed"), http.StatusInternalServerError)
		}
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func readJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := httpguard.DecodeRequestJSON(writer, request, target); err != nil {
		writeError(writer, err, http.StatusBadRequest)
		return false
	}
	return true
}

func writeError(writer http.ResponseWriter, err error, status int) {
	writeJSON(writer, map[string]string{"error": err.Error()}, status)
}

func writeJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func (handler api) identity(writer http.ResponseWriter, request *http.Request) {
	profile, allowed := handler.access.Profile(request)
	if !allowed || handler.manager.serverID == "" || request.URL.RawQuery != "" {
		writeError(writer, errors.New("download access required"), http.StatusForbidden)
		return
	}
	writeJSON(writer, map[string]string{"serverId": handler.manager.serverID, "profileId": profile}, http.StatusOK)
}

func (handler api) manifest(writer http.ResponseWriter, request *http.Request) {
	profile, allowed := handler.access.Profile(request)
	if !allowed || request.URL.RawQuery != "" {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	job, found := handler.manager.Get(profile, request.PathValue("id"))
	if !found || !visibleJob(handler.access, request, profile, job) {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	if job.State == "preparing" {
		writer.Header().Set("Retry-After", "15")
		writeError(writer, errors.New("download is preparing"), http.StatusServiceUnavailable)
		return
	}
	manifest, err := handler.manager.ManifestContext(request.Context(), profile, request.PathValue("id"))
	if err != nil {
		if errors.Is(err, ErrQueueFull) {
			writer.Header().Set("Retry-After", "15")
			writeError(writer, err, http.StatusServiceUnavailable)
			return
		}
		writeError(writer, errors.New("download manifest unavailable"), http.StatusNotFound)
		return
	}
	writeJSON(writer, manifest, http.StatusOK)
}

func (handler api) tracks(writer http.ResponseWriter, request *http.Request) {
	_, item, allowed := handler.access.Item(request, request.PathValue("id"))
	if !allowed || request.URL.RawQuery != "" {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	result, err := handler.manager.Tracks(item)
	if err != nil {
		writeError(writer, errors.New("track information is unavailable"), http.StatusBadRequest)
		return
	}
	writeJSON(writer, result, http.StatusOK)
}
