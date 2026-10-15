package apihttp

import "net/http"

type Application interface {
	RegisterFoundationAPI(*http.ServeMux)
	RegisterDownloadsAPI(*http.ServeMux)
	RegisterAdminAPI(*http.ServeMux)
	RegisterSupporterAPI(*http.ServeMux)
	RegisterProductAPI(*http.ServeMux)
}

func Register(mux *http.ServeMux, application Application) {
	application.RegisterFoundationAPI(mux)
	application.RegisterDownloadsAPI(mux)
	application.RegisterAdminAPI(mux)
	application.RegisterSupporterAPI(mux)
	application.RegisterProductAPI(mux)
}
