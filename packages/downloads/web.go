package downloads

import (
	"errors"
	"net/http"
	"os"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// Renderer executes one localized application template.
type Renderer interface {
	Execute(http.ResponseWriter, *http.Request, any) error
}

// View is the offline-download page model.
type View struct {
	Jobs    []Job
	Pending bool
	Profile string
}

type web struct {
	manager    *Manager
	access     Access
	view       Renderer
	writeError func(http.ResponseWriter, *http.Request, string, int)
	notFound   func(http.ResponseWriter, *http.Request)
}

// RegisterWeb registers Player's HTML download lifecycle.
func RegisterWeb(
	mux *http.ServeMux,
	manager *Manager,
	access Access,
	view Renderer,
	writeError func(http.ResponseWriter, *http.Request, string, int),
	notFound func(http.ResponseWriter, *http.Request),
) {
	handler := web{manager, access, view, writeError, notFound}
	mux.HandleFunc("GET /offline-downloads", handler.list)
	mux.HandleFunc("POST /offline/{id}", handler.start)
	mux.HandleFunc("POST /offline-downloads/{id}/remove", handler.remove)
}

func (handler web) list(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	if !allowed {
		handler.writeError(writer, request, "downloads are not enabled for this Viewer Profile", http.StatusForbidden)
		return
	}
	jobs := handler.manager.List(profileID)
	pending := false
	for _, job := range jobs {
		pending = pending || !job.ReadyOffline && job.Error == ""
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = handler.view.Execute(writer, request, View{jobs, pending, profileID})
}

func (handler web) start(writer http.ResponseWriter, request *http.Request) {
	profileID, item, found := handler.access.Item(request, request.PathValue("id"))
	if !found {
		handler.notFound(writer, request)
		return
	}
	if err := httpguard.DecodeForm(writer, request, 1024, "quality"); err != nil {
		handler.writeError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := handler.manager.Start(profileID, item, request.PostForm.Get("quality")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrQueueFull) {
			status = http.StatusServiceUnavailable
			writer.Header().Set("Retry-After", "15")
		}
		if errors.Is(err, ErrCapacity) {
			status = http.StatusConflict
		}
		handler.writeError(writer, request, err.Error(), status)
		return
	}
	http.Redirect(writer, request, "/offline-downloads", http.StatusSeeOther)
}

func (handler web) remove(writer http.ResponseWriter, request *http.Request) {
	profileID, allowed := handler.access.Profile(request)
	if !allowed {
		handler.notFound(writer, request)
		return
	}
	if err := httpguard.DecodeForm(writer, request, 1024); err != nil {
		handler.writeError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err := handler.manager.Remove(profileID, request.PathValue("id")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			handler.notFound(writer, request)
		} else {
			handler.writeError(writer, request, "download could not be removed", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(writer, request, "/offline-downloads", http.StatusSeeOther)
}
