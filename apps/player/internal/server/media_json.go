package server

import "net/http"

func readMediaJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	return readJSON(writer, request, target)
}
