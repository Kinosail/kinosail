package server

import (
	"html/template"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

var (
	csrfToken           = httpguard.CSRFToken
	executeCSRFTemplate = httpguard.ExecuteCSRFTemplate
)

func csrfParseFuncs() template.FuncMap { return httpguard.CSRFParseFuncs(uiIcon) }

func newCSRFTemplate(name, source string) *template.Template {
	return httpguard.NewCSRFTemplate(name, source, uiIcon)
}
