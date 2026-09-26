package server

import "net/http"

func apiConfiguration(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		fields := settings.configuration().Fields()
		for index := range fields {
			if fields[index].Key == "logging.level" && fields[index].Source != "environment" && fields[index].Source != "yaml" {
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
		value, expiresAt, ok := apiConfigurationInput(writer, request, key, reset)
		if !ok {
			return
		}
		if err := applyAPIConfiguration(settings, key, value, expiresAt, reset); err != nil {
			apiError(writer, err, http.StatusConflict)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": key != "logging.level"}, http.StatusAccepted)
	}
}

func apiChangeConfigurationGroup(writer http.ResponseWriter, request *http.Request, settings *settingsStore, key string, reset bool) bool {
	switch key {
	case tmdbConfigurationKey:
		apiChangeTMDBConfiguration(writer, request, settings, reset)
	case oidcConfigurationKey:
		apiChangeOIDCConfiguration(writer, request, settings, reset)
	case samlConfigurationGroupKey:
		apiChangeSAMLConfiguration(writer, request, settings, reset)
	case scimConfigurationKey:
		apiChangeSCIMConfiguration(writer, request, settings, reset)
	default:
		return false
	}
	return true
}

func apiConfigurationInput(writer http.ResponseWriter, request *http.Request, key string, reset bool) (string, string, bool) {
	if reset {
		return "", "", true
	}
	if key == "integrations.scim.token" {
		var input struct {
			Value     string `json:"value"`
			ExpiresAt string `json:"tokenExpiresAt"`
		}
		ok := readJSON(writer, request, &input)
		return input.Value, input.ExpiresAt, ok
	}
	var input struct {
		Value string `json:"value"`
	}
	ok := readJSON(writer, request, &input)
	return input.Value, "", ok
}

func applyAPIConfiguration(settings *settingsStore, key, value, expiresAt string, reset bool) error {
	if key == "integrations.scim.token" && !reset {
		return settings.changeSCIMConfiguration(value, expiresAt, false)
	}
	return settings.changeConfiguration(key, value, reset)
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
	writeConfigurationSaved(writer)
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
	writeConfigurationSaved(writer)
}

func apiChangeSCIMConfiguration(writer http.ResponseWriter, request *http.Request, settings *settingsStore, reset bool) {
	var input struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"tokenExpiresAt"`
	}
	if !reset && !readJSON(writer, request, &input) {
		return
	}
	if err := settings.changeSCIMConfiguration(input.Token, input.ExpiresAt, reset); err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeConfigurationSaved(writer)
}

func writeConfigurationSaved(writer http.ResponseWriter) {
	writeJSON(writer, map[string]any{"status": "saved", "restartRequired": true}, http.StatusAccepted)
}
