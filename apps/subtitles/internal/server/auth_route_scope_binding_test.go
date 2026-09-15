package server

import "github.com/MikeO7/kinosail/packages/servertest"

func expectedAPIKeyScopes(pattern string) map[string]bool {
	return servertest.ExpectedRouteAPIKeyScopes(pattern, sessionOnlyAPIRoutes, map[string]map[string]bool{
		"library": expectedLibraryScopeRoutes, "write": expectedWriteScopeRoutes, "stream": expectedStreamScopeRoutes, "download": expectedDownloadScopeRoutes,
	})
}
