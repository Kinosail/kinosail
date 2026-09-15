package apihttp

import "net/http"

// Application supplies product-owned adapters to Player's API route order.
type Application interface {
	RegisterFoundationAPI(*http.ServeMux)
	RegisterDownloadsAPI(*http.ServeMux)
	RegisterAdminAPI(*http.ServeMux)
	RegisterSupporterAPI(*http.ServeMux)
	RegisterProductAPI(*http.ServeMux)
}

// Register installs Player's canonical top-level API groups.
func Register(mux *http.ServeMux, application Application) {
	application.RegisterFoundationAPI(mux)
	application.RegisterDownloadsAPI(mux)
	application.RegisterAdminAPI(mux)
	application.RegisterSupporterAPI(mux)
	application.RegisterProductAPI(mux)
}
