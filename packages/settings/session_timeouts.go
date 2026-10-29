package settings

import (
	"errors"
	"net/http"
)

func SaveSessionTimeoutsAPI(change func(inactive, absolute float64) error, invalid error, failed func(*http.Request), read func(http.ResponseWriter, *http.Request, any) bool, write func(http.ResponseWriter, any, int), writeError func(http.ResponseWriter, error, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ InactiveHours, AbsoluteHours float64 }
		if !read(writer, request, &input) {
			return
		}
		if err := change(input.InactiveHours, input.AbsoluteHours); err != nil {
			status := http.StatusBadRequest
			if !errors.Is(err, invalid) {
				failed(request)
				err = errors.New("could not save session timeouts")
				status = http.StatusInternalServerError
			}
			writeError(writer, err, status)
			return
		}
		write(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}
