package transcodepolicy

import (
	"context"
	"net/http"
)

// CheckHandler runs one explicit check and returns to the transcoder settings.
func CheckHandler(run func(context.Context) CheckResult) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		run(request.Context())
		http.Redirect(writer, request, "/settings#transcoder", http.StatusSeeOther)
	}
}
