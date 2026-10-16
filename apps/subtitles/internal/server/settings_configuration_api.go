package server

import "net/http"

func apiConfiguration(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		fields := settings.configuration().Fields()
		for index := range fields {
			if (subtitleProviderSetting(fields[index].Key) || fields[index].Key == "logging.level") && fields[index].Source != "environment" && fields[index].Source != "yaml" {
				fields[index].Restart = false
			}
		}
		writeJSON(writer, map[string]any{"settings": fields}, http.StatusOK)
	}
}

func apiChangeConfiguration(settings *settingsStore, reset bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		key := request.PathValue("key")
		if apiChangeConfigurationGroup(writer, request, settings, key, reset) {
			return
		}
		value, ok := apiConfigurationInput(writer, request, reset)
		if !ok {
			return
		}
		if err := settings.changeConfiguration(key, value, reset); err != nil {
			apiError(writer, err, http.StatusConflict)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": !subtitleProviderSetting(key) && key != "logging.level" && len(settings.pendingRestart()) > 0}, http.StatusAccepted)
	}
}

func apiConfigurationInput(writer http.ResponseWriter, request *http.Request, reset bool) (string, bool) {
	if reset {
		return "", true
	}
	var input struct {
		Value string `json:"value"`
	}
	ok := readJSON(writer, request, &input)
	return input.Value, ok
}

func apiChangeConfigurationGroup(writer http.ResponseWriter, request *http.Request, settings *settingsStore, key string, reset bool) bool {
	switch key {
	case oidcConfigurationKey:
		apiChangeOIDCConfiguration(writer, request, settings, reset)
	case openSubtitlesConfigurationKey:
		apiChangeOpenSubtitlesConfiguration(writer, request, settings, reset)
	case samlConfigurationGroupKey:
		apiChangeSAMLConfiguration(writer, request, settings, reset)
	case subSourceConfigurationKey:
		apiChangeSubSourceConfiguration(writer, request, settings, reset)
	default:
		return false
	}
	return true
}

func apiChangeOIDCConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		Issuer        string `json:"issuer"`
		ClientID      string `json:"clientId"`
		ClientSecret  string `json:"clientSecret"`
		RedirectURL   string `json:"redirectUrl"`
		IdentityClaim string `json:"identityClaim"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if err := settings.changeOIDCConfiguration(input.Issuer, input.ClientID, input.ClientSecret, input.RedirectURL, input.IdentityClaim, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, map[string]any{"status": "saved", "restartRequired": len(settings.pendingRestart()) > 0}, http.StatusAccepted)
}

func apiChangeSAMLConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		MetadataURL       string `json:"metadataUrl"`
		MetadataXML       string `json:"metadataXml"`
		IdentityAttribute string `json:"identityAttribute"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if err := settings.changeSAMLConfiguration(input.MetadataURL, input.MetadataXML, input.IdentityAttribute, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, map[string]any{"status": "saved", "restartRequired": len(settings.pendingRestart()) > 0}, http.StatusAccepted)
}
