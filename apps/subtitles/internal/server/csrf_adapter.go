package server

import (
	"html/template"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

var (
	csrfToken           = httpguard.CSRFToken
	executeCSRFTemplate = httpguard.ExecuteCSRFTemplate
)

func newCSRFTemplate(name, source string) *template.Template {
	return httpguard.NewCSRFTemplate(name, versionAssetReferences(source), uiIcon)
}

func httpguardTemplateSource(source string) string {
	return httpguard.CSRFTemplateSource(source)
}
