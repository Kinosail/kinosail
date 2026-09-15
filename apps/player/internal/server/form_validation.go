package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func strictSingleValueForm(request *http.Request, keys ...string) bool {
	if request.URL.RawQuery != "" || !httpguard.FormEncoded(request) || request.ParseForm() != nil || !httpguard.OnlyFormKeys(request.PostForm, keys...) {
		return false
	}
	for _, values := range request.PostForm {
		if len(values) != 1 {
			return false
		}
	}
	return true
}
