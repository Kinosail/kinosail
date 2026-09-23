package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

var errSupporterDisplay = errors.New("choose automatic, Living Standard, Patron Order, or hidden recognition")

func validSupporterDisplay(value string) bool {
	return value == "automatic" || value == livingStandardFamily || value == patronOrderFamily || value == "hidden"
}

func (store *settingsStore) supporterDisplay() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if validSupporterDisplay(store.value.SupporterDisplay) {
		return store.value.SupporterDisplay
	}
	return "automatic"
}

func (store *settingsStore) setSupporterDisplay(value string) error {
	if !validSupporterDisplay(value) {
		return errSupporterDisplay
	}
	return store.changeInstallationSettings(func(settings *installationSettings) { settings.SupporterDisplay = value })
}

func (page supporterPageData) Featured() *supporterOwnedBadgeView {
	if page.Monthly != nil {
		return page.Monthly
	}
	if page.Yearly != nil {
		return page.Yearly
	}
	if page.Display == patronOrderFamily && page.Patron != nil {
		return page.Patron
	}
	if page.Living != nil && (page.Living.Active || page.Display == livingStandardFamily) {
		return page.Living
	}
	if page.Patron != nil {
		return page.Patron
	}
	return page.Living
}

func registerSupporterDisplay(mux *http.ServeMux, auth *authentication, program *supporterProgram) {
	mux.Handle("GET /api/v1/supporter/display", auth.owner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			apiError(w, errSupporterDisplay, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"display": program.settings.supporterDisplay()}, http.StatusOK)
	})))
	mux.Handle("PUT /api/v1/supporter/display", auth.owner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value string
		r.Body = http.MaxBytesReader(w, r.Body, 128)
		if !readSupporterDisplay(r, &value) {
			apiError(w, errSupporterDisplay, http.StatusBadRequest)
			return
		}
		if err := program.settings.setSupporterDisplay(value); err != nil {
			apiError(w, errors.New("could not save supporter display"), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"display": value}, http.StatusOK)
	})))
	mux.Handle("POST /supporter/display", auth.owner(http.HandlerFunc(program.saveSupporterDisplay)))
}

func (program *supporterProgram) saveSupporterDisplay(writer http.ResponseWriter, request *http.Request) {
	fail := func(message string, status int) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(status)
		_ = supporterView.Execute(writer, request, program.pageData(message))
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 128)
	if !httpguard.FormEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !httpguard.OnlyFormKeys(request.PostForm, "display") {
		fail(errSupporterDisplay.Error(), http.StatusBadRequest)
		return
	}
	value, ok := httpguard.RequiredValue(request.PostForm, "display", 32)
	if !ok || !validSupporterDisplay(value) {
		fail(errSupporterDisplay.Error(), http.StatusBadRequest)
		return
	}
	if err := program.settings.setSupporterDisplay(value); err != nil {
		fail("Could not save your display choice. Please try again.", http.StatusInternalServerError)
		return
	}
	http.Redirect(writer, request, "/supporter#recognition", http.StatusSeeOther)
}

func readSupporterDisplay(r *http.Request, value *string) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || r.URL.RawQuery != "" {
		return false
	}
	return decodeSupporterDisplay(r.Body, value)
}

func decodeSupporterDisplay(reader io.Reader, value *string) bool {
	decoder := json.NewDecoder(reader)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return false
	}
	field, err := decoder.Token()
	if err != nil || field != "display" || decoder.Decode(value) != nil || !validSupporterDisplay(*value) {
		return false
	}
	closing, err := decoder.Token()
	return err == nil && closing == json.Delim('}') && decoder.Decode(&struct{}{}) == io.EOF
}
