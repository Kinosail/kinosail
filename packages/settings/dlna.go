package settings

import (
	"crypto/rand"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/dlna"
)

func ConfiguredDLNAToken(configured, enabled bool, current string) (string, bool) {
	if !configured {
		return current, false
	}
	if !enabled {
		return "", true
	}
	if current == "" {
		current = rand.Text()
	}
	return current, true
}

func NewDLNAToken(enabled bool, address string) (string, error) {
	if !enabled {
		return "", nil
	}
	if !dlna.ValidURL(address) {
		return "", errors.New("KINOSAIL_DLNA_URL is required")
	}
	return rand.Text(), nil
}

func DLNALabel(address, token string) string {
	if address == "" {
		return "Set KINOSAIL_DLNA_URL to enable"
	}
	if token != "" {
		return "Enabled"
	}
	return "Disabled"
}

func SaveDLNA(change func(bool) error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := decodeForm(writer, request, 1024, map[string]int{"enabled": 1}); err != nil {
			writeError(writer, request, "DLNA setting is invalid", http.StatusBadRequest)
			return
		}
		raw := request.PostForm.Get("enabled")
		if raw != "true" && raw != "false" {
			writeError(writer, request, "DLNA setting is invalid", http.StatusBadRequest)
			return
		}
		if err := change(raw == "true"); err != nil {
			writeError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}
