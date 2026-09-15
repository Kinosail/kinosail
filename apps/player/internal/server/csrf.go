package server

import (
	"html/template"
	"net/http"
	"regexp"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

var (
	formStart = regexp.MustCompile(`(?i)<form(?:\s[^>]*)?>`)
	postForm  = regexp.MustCompile(`(?i)(?:^|\s)method\s*=\s*["']?post["']?(?:\s|>|$)`)
)

func csrfTemplateSource(source string) string {
	source = formStart.ReplaceAllStringFunc(source, func(form string) string {
		if !postForm.MatchString(form) {
			return form
		}
		return form + `{{csrfField}}`
	})
	return regexp.MustCompile(`(?i)</head>`).ReplaceAllString(source, `{{csrfMeta}}</head>`)
}

func csrfTemplateFuncs(request *http.Request) template.FuncMap {
	return template.FuncMap{
		"csrfField": func() template.HTML {
			if token := csrfForRequest(request); token != "" {
				return template.HTML(`<input type="hidden" name="_csrf" value="` + token + `">`) //nolint:gosec // SHA-256 base64url tokens cannot contain HTML metacharacters.
			}
			return ""
		},
		"csrfMeta": func() template.HTML {
			if token := csrfForRequest(request); token != "" {
				return template.HTML(`<meta name="kinosail-csrf" content="` + token + `">`) //nolint:gosec // SHA-256 base64url tokens cannot contain HTML metacharacters.
			}
			return ""
		},
	}
}

func csrfParseFuncs() template.FuncMap {
	return template.FuncMap{"csrfField": func() template.HTML { return "" }, "csrfMeta": func() template.HTML { return "" }, "icon": uiIcon}
}

func newCSRFTemplate(name, source string) *template.Template {
	return template.Must(template.New(name).Funcs(csrfParseFuncs()).Parse(csrfTemplateSource(source)))
}

func executeCSRFTemplate(view *template.Template, writer http.ResponseWriter, request *http.Request, data any) error {
	page, err := view.Clone()
	if err != nil {
		return err
	}
	page.Funcs(csrfTemplateFuncs(request))
	return page.Execute(writer, data)
}

var (
	csrfForRequest = httpguard.CSRFForRequest
	csrfToken      = httpguard.CSRFToken
	validCSRF      = httpguard.ValidCSRF
)
