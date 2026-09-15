package identitycore

import (
	"net/http"
	"net/url"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	viewerMutationFormLimit = 4096
	viewerPasswordLimit     = 1024
	invalidViewerProfile    = "invalid Viewer Profile"
)

type viewerFormMutation func(url.Values) (invalid string, err error)

// ViewerPasswordResetHandler applies Player's bounded Viewer password reset HTTP contract.
func ViewerPasswordResetHandler(reset func(string, string) error, writeError func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	if reset == nil {
		return unavailableViewerMutationHandler()
	}
	return viewerMutationHandler([]string{"id", "password"}, "/settings", func(values url.Values) (string, error) {
		id, validID := httpguard.RequiredValue(values, "id", maxIdentityLength)
		password, validPassword := httpguard.RequiredValue(values, "password", viewerPasswordLimit)
		if !validID || !validPassword {
			return invalidViewerProfile, nil
		}
		return "", reset(id, password)
	}, writeError)
}

// ViewerRemovalHandler applies Player's bounded Viewer removal HTTP contract.
func ViewerRemovalHandler(remove func(string) error, writeError func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	if remove == nil {
		return unavailableViewerMutationHandler()
	}
	return viewerMutationHandler([]string{"id"}, "/settings", func(values url.Values) (string, error) {
		id, valid := httpguard.RequiredValue(values, "id", maxIdentityLength)
		if !valid {
			return invalidViewerProfile, nil
		}
		return "", remove(id)
	}, writeError)
}

func viewerMutationHandler(keys []string, redirect string, mutate viewerFormMutation, writeError func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	if writeError == nil {
		return unavailableViewerMutationHandler()
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := httpguard.DecodeForm(writer, request, viewerMutationFormLimit, keys...); err != nil {
			writeError(writer, request, invalidViewerProfile, http.StatusBadRequest)
			return
		}
		message, err := mutate(request.PostForm)
		if message != "" {
			writeError(writer, request, message, http.StatusBadRequest)
			return
		}
		if err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, redirect, http.StatusSeeOther)
	}
}

func unavailableViewerMutationHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "Viewer profile mutation is unavailable", http.StatusInternalServerError)
	}
}
