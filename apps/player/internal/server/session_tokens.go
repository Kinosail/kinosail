package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func sessionToken(request *http.Request) string {
	return identitycore.SessionToken(request, jellyfinSessionQueryToken)
}

func jellyfinSessionQueryToken(request *http.Request) (string, string) {
	return jellyfinMediaQueryToken(request), "jellyfin-api-key-query"
}

func jellyfinMediaQueryToken(request *http.Request) string {
	if !jellyfinMediaPath(request.URL.Path) {
		return ""
	}
	values, present := request.URL.Query()["api_key"]
	if !present || len(values) != 1 || len(values[0]) == 0 || len(values[0]) > 256 {
		return ""
	}
	return values[0]
}

func jellyfinMediaQueryTokenPresent(request *http.Request) bool {
	if !jellyfinMediaPath(request.URL.Path) {
		return false
	}
	_, present := request.URL.Query()["api_key"]
	return present
}

func jellyfinMediaPath(path string) bool {
	return strings.HasPrefix(path, "/Videos/") || strings.HasPrefix(path, "/Audio/")
}

func sessionTokenSource(request *http.Request) string {
	return identitycore.SessionTokenSource(request, jellyfinSessionQueryToken)
}

func sessionKey(token string) string { return identitycore.SessionKey(token) }

func cleanDeviceName(name string) string { return identitycore.CleanDeviceName(name) }
