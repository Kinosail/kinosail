package catalog

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

type loadingResponse struct {
	http.ResponseWriter
	started bool
}

func (response *loadingResponse) WriteHeader(status int) {
	response.started = true
	response.ResponseWriter.WriteHeader(status)
}

func (response *loadingResponse) Write(data []byte) (int, error) {
	if !response.started {
		response.WriteHeader(http.StatusServiceUnavailable)
	}
	return response.ResponseWriter.Write(data)
}

func (response *loadingResponse) Unwrap() http.ResponseWriter {
	return response.ResponseWriter
}

func LoadingResponse(writer http.ResponseWriter) http.ResponseWriter {
	library.MarkLoadingRetry(writer.Header(), library.ErrLoading)
	return &loadingResponse{ResponseWriter: writer}
}
