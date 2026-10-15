package httpguard

import (
	"html/template"
	"net/http"
	"regexp"
)

var (
	csrfFormStart = regexp.MustCompile(`(?i)<form(?:\s[^>]*)?>`)
	csrfPostForm  = regexp.MustCompile(`(?i)(?:^|\s)method\s*=\s*["']?post["']?(?:\s|>|$)`)
	csrfHeadClose = regexp.MustCompile(`(?i)</head>`)
)

func CSRFTemplateSource(source string) string {
	source = csrfFormStart.ReplaceAllStringFunc(source, func(form string) string {
		if !csrfPostForm.MatchString(form) {
			return form
		}
		return form + `{{csrfField}}`
	})
	return csrfHeadClose.ReplaceAllString(source, `{{csrfMeta}}</head>`)
}

func CSRFTemplateFuncs(request *http.Request) template.FuncMap {
	return template.FuncMap{
		"csrfField": func() template.HTML {
			if token := CSRFForRequest(request); token != "" {
				return template.HTML(`<input type="hidden" name="_csrf" value="` + token + `">`) //nolint:gosec // SHA-256 base64url tokens cannot contain HTML metacharacters.
			}
			return ""
		},
		"csrfMeta": func() template.HTML {
			if token := CSRFForRequest(request); token != "" {
				return template.HTML(`<meta name="kinosail-csrf" content="` + token + `">`) //nolint:gosec // SHA-256 base64url tokens cannot contain HTML metacharacters.
			}
			return ""
		},
	}
}

func CSRFParseFuncs(icon func(string) template.HTML) template.FuncMap {
	return template.FuncMap{
		"csrfField": func() template.HTML { return "" },
		"csrfMeta":  func() template.HTML { return "" },
		"icon":      icon,
	}
}

func NewCSRFTemplate(name, source string, icon func(string) template.HTML) *template.Template {
	return template.Must(template.New(name).Funcs(CSRFParseFuncs(icon)).Parse(CSRFTemplateSource(source)))
}

func ExecuteCSRFTemplate(view *template.Template, writer http.ResponseWriter, request *http.Request, data any) error {
	page, err := view.Clone()
	if err != nil {
		return err
	}
	page.Funcs(CSRFTemplateFuncs(request))
	return page.Execute(writer, data)
}
