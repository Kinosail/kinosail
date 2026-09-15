package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func emptyMutationRequest(writer http.ResponseWriter, request *http.Request) bool {
	return httpguard.EmptyMutationRequest(writer, request)
}
