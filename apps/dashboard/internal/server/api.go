package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/MikeO7/kinosail-dashboard/internal/supporter"
)

func registerAPI(mux *http.ServeMux, board *dashboard.Service, prober *dashboard.Prober, supporterProgram *supporter.Service) {
	registerAPIInfo(mux)
	registerBoardAPI(mux, board)
	registerAppCreateAPI(mux, board)
	registerAppManageAPI(mux, board)
	registerProbeAPI(mux, board, prober)
	registerImportAPI(mux, board)
	registerSupporterAPI(mux, supporterProgram)
}

func registerAPIInfo(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		writeJSON(writer, map[string]any{"name": "Kinosail Dashboard API", "version": "v1", "openapi": "/api/v1/openapi.json"}, http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/openapi.json", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		serveFile("web/openapi.json", "application/json; charset=utf-8")(writer, request)
	})
	mux.HandleFunc("GET /api/v1/me", func(writer http.ResponseWriter, request *http.Request) {
		identity, found := requireOwner(writer, request)
		if !found {
			return
		}
		writeJSON(writer, map[string]any{"owner": map[string]string{"id": identity.ID, "name": identity.Name}, "csrf": identity.CSRF}, http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/catalog", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		writeJSON(writer, map[string]any{"apps": dashboard.Catalog(), "aliases": dashboard.CatalogAliases()}, http.StatusOK)
	})
}

func registerBoardAPI(mux *http.ServeMux, board *dashboard.Service) {
	mux.HandleFunc("GET /api/v1/board", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		writeJSON(writer, board.Snapshot(), http.StatusOK)
	})
	mux.HandleFunc("PUT /api/v1/board", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct {
			Title           string `json:"title"`
			ExpectedVersion uint64 `json:"expectedVersion"`
		}
		if !readJSON(writer, request, &input, 16<<10) {
			return
		}
		receipt, err := board.Rename(request.Context(), input.Title, input.ExpectedVersion, actor(identity))
		writeMutation(writer, receipt, err)
	})
}

func registerAppCreateAPI(mux *http.ServeMux, board *dashboard.Service) {
	mux.HandleFunc("POST /api/v1/apps", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input dashboard.CreateInput
		if !readJSON(writer, request, &input, 32<<10) {
			return
		}
		app, receipt, err := board.Create(request.Context(), input, actor(identity))
		if err != nil {
			writeDomainError(writer, err)
			return
		}
		writeJSON(writer, map[string]any{"app": app, "receipt": receipt}, http.StatusCreated)
	})
	mux.HandleFunc("PATCH /api/v1/apps/{id}", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input dashboard.UpdateInput
		if !readJSON(writer, request, &input, 32<<10) {
			return
		}
		app, receipt, err := board.Update(request.Context(), request.PathValue("id"), input, actor(identity))
		if err != nil {
			writeDomainError(writer, err)
			return
		}
		writeJSON(writer, map[string]any{"app": app, "receipt": receipt}, http.StatusOK)
	})
}

func registerAppManageAPI(mux *http.ServeMux, board *dashboard.Service) {
	mux.HandleFunc("DELETE /api/v1/apps/{id}", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct {
			ExpectedVersion uint64 `json:"expectedVersion"`
		}
		if !readJSON(writer, request, &input, 8<<10) {
			return
		}
		receipt, err := board.Remove(request.Context(), request.PathValue("id"), input.ExpectedVersion, actor(identity))
		writeMutation(writer, receipt, err)
	})
	mux.HandleFunc("POST /api/v1/apps/{id}/restore", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct {
			ExpectedVersion uint64 `json:"expectedVersion"`
		}
		if !readJSON(writer, request, &input, 8<<10) {
			return
		}
		app, receipt, err := board.Restore(request.Context(), request.PathValue("id"), input.ExpectedVersion, actor(identity))
		if err != nil {
			writeDomainError(writer, err)
			return
		}
		writeJSON(writer, map[string]any{"app": app, "receipt": receipt}, http.StatusOK)
	})
	mux.HandleFunc("PUT /api/v1/apps/order", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct {
			IDs             []string `json:"ids"`
			ExpectedVersion uint64   `json:"expectedVersion"`
		}
		if !readJSON(writer, request, &input, 32<<10) {
			return
		}
		receipt, err := board.Reorder(request.Context(), input.IDs, input.ExpectedVersion, actor(identity))
		writeMutation(writer, receipt, err)
	})
}

func registerProbeAPI(mux *http.ServeMux, board *dashboard.Service, prober *dashboard.Prober) {
	mux.HandleFunc("POST /api/v1/apps/{id}/check", func(writer http.ResponseWriter, request *http.Request) {
		_, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct{}
		if !readJSON(writer, request, &input, 1024) {
			return
		}
		health, err := prober.ProbeOne(request.Context(), request.PathValue("id"))
		if err != nil {
			writeDomainError(writer, err)
			return
		}
		writeJSON(writer, map[string]any{"health": health}, http.StatusOK)
	})
	mux.HandleFunc("POST /api/v1/board/check", func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := authorizedMutation(writer, request); !ok {
			return
		}
		var input struct{}
		if !readJSON(writer, request, &input, 1024) {
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
		defer cancel()
		check := prober.ProbeAll(ctx)
		writeJSON(writer, map[string]any{"board": board.Snapshot(), "check": check}, http.StatusOK)
	})
}

func registerImportAPI(mux *http.ServeMux, board *dashboard.Service) { //nolint:gocognit // Import routes stay below the quality ceiling.
	mux.HandleFunc("GET /api/v1/export", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-dashboard-backup.json"`)
		exported := board.BoardExport()
		writeJSON(writer, map[string]any{"title": exported.Title, "apps": exported.Apps}, http.StatusOK)
	})
	mux.HandleFunc("PUT /api/v1/import", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input dashboard.Import
		if !readJSON(writer, request, &input, 2<<20) {
			return
		}
		receipt, err := board.ImportBoard(request.Context(), input, actor(identity))
		writeMutation(writer, receipt, err)
	})
	mux.HandleFunc("POST /api/v1/import/preview", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		var input dashboard.ExternalImport
		if !readJSON(writer, request, &input, 3<<20) {
			return
		}
		preview, err := dashboard.PreviewExternal(input)
		if err != nil {
			writeDomainError(writer, err)
			return
		}
		writeJSON(writer, preview, http.StatusOK)
	})
	mux.HandleFunc("POST /api/v1/import/external", func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := authorizedMutation(writer, request)
		if !ok {
			return
		}
		var input struct {
			dashboard.ExternalImport
			ExpectedVersion uint64 `json:"expectedVersion"`
		}
		if !readJSON(writer, request, &input, 3<<20) {
			return
		}
		receipt, err := board.ImportExternal(request.Context(), input.ExternalImport, input.ExpectedVersion, actor(identity))
		writeMutation(writer, receipt, err)
	})
}

func registerSupporterAPI(mux *http.ServeMux, supporterProgram *supporter.Service) {
	mux.HandleFunc("GET /api/v1/supporter", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := requireOwner(writer, request); !found {
			return
		}
		writeJSON(writer, supporterProgram.Status(), http.StatusOK)
	})
	mux.HandleFunc("POST /api/v1/supporter/activate", func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := authorizedMutation(writer, request); !ok {
			return
		}
		var input supporter.ActivationInput
		if !readJSON(writer, request, &input, 8<<10) {
			return
		}
		status, err := supporterProgram.Activate(request.Context(), input)
		if err != nil {
			code := http.StatusBadRequest
			switch {
			case errors.Is(err, supporter.ErrConflict):
				code = http.StatusConflict
			case errors.Is(err, supporter.ErrUncertain):
				code = http.StatusBadGateway
			case errors.Is(err, supporter.ErrUnavailable):
				code = http.StatusServiceUnavailable
			}
			apiError(writer, err, code)
			return
		}
		writeJSON(writer, status, http.StatusOK)
	})
}
