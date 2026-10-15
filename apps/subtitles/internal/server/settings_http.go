package server

import "net/http"

const settingControlHTML = `{{define "configurationSource"}}{{if .Managed}}<p class="managed-setting"><strong>Read-only.</strong>{{if .DockerVars}} Configured via Docker: {{range $index, $name := .DockerVars}}{{if $index}}, {{end}}<code>{{$name}}</code>{{end}}.{{end}}{{if .YAMLKeys}} Configured via YAML: {{range $index, $name := .YAMLKeys}}{{if $index}}, {{end}}<code>{{$name}}</code>{{end}}.{{end}}</p>{{end}}{{if .ConnectionURL}}<p>Connect address: <code>{{.ConnectionURL}}</code></p>{{end}}{{end}}`

func finishOnboarding(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.setOnboardingPending(false); err != nil {
			localizedError(writer, request, "could not finish onboarding", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}

func restartOnboarding(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.setOnboardingPending(true); err != nil {
			localizedError(writer, request, "could not restart onboarding", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/onboarding", http.StatusSeeOther)
	}
}
