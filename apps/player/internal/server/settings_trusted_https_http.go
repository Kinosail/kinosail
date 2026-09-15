package server

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

type trustedHTTPSRequest struct {
	Provider, Domain, Token, Address string
	TermsAccepted                    bool `json:"termsAccepted"`
}

func (input trustedHTTPSRequest) settingsInput() trustedHTTPSInput {
	return trustedHTTPSInput(input)
}

func apiTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input trustedHTTPSRequest
		if !readJSON(writer, request, &input) {
			return
		}
		view, err := settings.setTrustedHTTPS(input.settingsInput())
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) || errors.Is(err, errTrustedHTTPSConflict) {
				status = http.StatusConflict
			} else if errors.Is(err, errTrustedHTTPSStorage) {
				status = http.StatusInternalServerError
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": true, "trustedHttps": view}, http.StatusAccepted)
	}
}

func apiTestTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input trustedHTTPSRequest
		if !readJSON(writer, request, &input) {
			return
		}
		view, err := settings.testTrustedHTTPS(request.Context(), input.settingsInput())
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) || errors.Is(err, errTrustedHTTPSConflict) {
				status = http.StatusConflict
			} else if errors.Is(err, errTrustedHTTPSCheck) {
				status = http.StatusBadGateway
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "passed", "trustedHttps": view}, http.StatusOK)
	}
}

func apiValidateTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input trustedHTTPSRequest
		if !readJSON(writer, request, &input) {
			return
		}
		config, err := settings.validateTrustedHTTPS(input.settingsInput())
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) || errors.Is(err, errTrustedHTTPSConflict) {
				status = http.StatusConflict
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "valid", "trustedHttps": trustedHTTPSConfigView(config)}, http.StatusOK)
	}
}

func apiDisableTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if err := settings.disableTrustedHTTPS(); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errManagedSetting) {
				status = http.StatusConflict
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": true}, http.StatusAccepted)
	}
}

func saveTrustedHTTPS(settings *settingsStore, redirect string) http.HandlerFunc {
	return trustedhttps.SettingsForm{
		Save: func(input trustedhttps.SettingsInput) error {
			_, err := settings.setTrustedHTTPS(trustedHTTPSInput(input))
			return err
		},
		Managed: errManagedSetting, Conflict: errTrustedHTTPSConflict, Storage: errTrustedHTTPSStorage,
		Error: localizedError, Redirect: redirect,
	}.ServeHTTP
}

func disableTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.disableTrustedHTTPS(); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errManagedSetting) {
				status = http.StatusConflict
			}
			localizedError(writer, request, err.Error(), status)
			return
		}
		http.Redirect(writer, request, "/settings#trusted-https", http.StatusSeeOther)
	}
}
