package transcodepolicy

import (
	"context"
	"net/http"
)

func CheckHandler(run func(context.Context) CheckResult) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		run(request.Context())
		http.Redirect(writer, request, "/settings#transcoder", http.StatusSeeOther)
	}
}
