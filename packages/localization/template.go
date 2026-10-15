package localization

import (
	"html/template"
	"net/http"
	"strings"
)

type TemplateRuntime struct {
	PreferredLanguage  func(*http.Request) string
	LanguagePreference func(*http.Request) string
	MatchLanguage      func(string) (string, bool)
	SetLanguageHeaders func(http.ResponseWriter, string)
	CSRFForRequest     func(*http.Request) string
	CSRFTemplateFuncs  func(*http.Request) template.FuncMap
}

type TemplateSet struct {
	views   map[string]*template.Template
	catalog *Catalog
	runtime TemplateRuntime
}

func NewTemplateSet(name, source, scriptVersion string, catalog *Catalog, languages []Language, preprocess func(string) string, parseFuncs template.FuncMap, runtime TemplateRuntime) *TemplateSet {
	if strings.Contains(source, `rel="stylesheet" href="/static/app.css`) && !strings.Contains(source, `class="auth`) && !strings.Contains(source, "/static/main.kinosail.bundle.js") {
		source = strings.Replace(source, "</head>", `<script defer src="/static/main.kinosail.bundle.js?v=`+scriptVersion+`"></script></head>`, 1)
	}
	source = preprocess(source)
	views := make(map[string]*template.Template, len(languages))
	for _, supported := range languages {
		tag := supported.Tag
		views[tag] = template.Must(template.New(name).Funcs(template.FuncMap{
			"t":              func(value string) string { return catalog.Localize(tag, value) },
			"languagePicker": func() template.HTML { return catalog.PickerHTML(tag, tag, "", false) },
		}).Funcs(parseFuncs).Parse(catalog.TranslateSource(source, tag)))
	}
	return &TemplateSet{views: views, catalog: catalog, runtime: runtime}
}

func (view *TemplateSet) Execute(writer http.ResponseWriter, request *http.Request, data any) error {
	return view.runtime.ExecuteTemplate(view.views, view.catalog, writer, request, "", data)
}

func (view *TemplateSet) ExecuteTemplate(writer http.ResponseWriter, request *http.Request, name string, data any) error {
	return view.runtime.ExecuteTemplate(view.views, view.catalog, writer, request, name, data)
}

func (runtime TemplateRuntime) ExecuteTemplate(views map[string]*template.Template, catalog *Catalog, writer http.ResponseWriter, request *http.Request, name string, data any) error {
	tag, preference := runtime.PreferredLanguage(request), runtime.LanguagePreference(request)
	runtime.SetLanguageHeaders(writer, tag)
	if selected, ok := runtime.MatchLanguage(request.URL.Query().Get("lang")); ok {
		preference = selected
	}
	page, err := views[tag].Clone()
	if err != nil {
		return err
	}
	picker := func() template.HTML {
		return catalog.PickerHTML(tag, preference, runtime.CSRFForRequest(request), false)
	}
	if request.URL.Path == "/" {
		picker = func() template.HTML {
			return catalog.PickerHTML(tag, preference, runtime.CSRFForRequest(request), true)
		}
	}
	page.Funcs(template.FuncMap{"languagePicker": picker}).Funcs(runtime.CSRFTemplateFuncs(request))
	if name != "" {
		return page.ExecuteTemplate(writer, name, data)
	}
	return page.Execute(writer, data)
}
