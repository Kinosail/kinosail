package localization

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

type TemplateRuntime struct {
	PreferredLanguage  func(*http.Request) string
	LanguagePreference func(*http.Request) string
	MatchLanguage      func(string) (string, bool)
	SetLanguageHeaders func(http.ResponseWriter, string)
	CSRFForRequest     func(*http.Request) string
	RequestHTML        map[string]func(*http.Request) template.HTML
}

type TemplateSet struct {
	name       string
	catalog    *Catalog
	runtime    TemplateRuntime
	parseFuncs template.FuncMap
	source     func() string
	views      func() []func() (*template.Template, error)
}

func NewTemplateSet(name string, source func() string, scriptVersion string, catalog *Catalog, preprocess func(string) string, parseFuncs template.FuncMap, runtime TemplateRuntime) *TemplateSet {
	view := &TemplateSet{name: name, catalog: catalog, runtime: runtime, parseFuncs: parseFuncs}
	view.source = sync.OnceValue(func() string {
		return preprocess(withPageScript(source(), scriptVersion))
	})
	view.views = sync.OnceValue(func() []func() (*template.Template, error) {
		views := make([]func() (*template.Template, error), catalog.groupCount())
		for group := range views {
			views[group] = sync.OnceValues(func() (*template.Template, error) { return view.parse(group) })
		}
		return views
	})
	return view
}

func withPageScript(source, scriptVersion string) string {
	if strings.Contains(source, `rel="stylesheet" href="/static/app.css`) && !strings.Contains(source, `class="auth`) && !strings.Contains(source, "/static/main.kinosail.bundle.js") {
		return strings.Replace(source, "</head>", `<script defer src="/static/main.kinosail.bundle.js?v=`+scriptVersion+`"></script></head>`, 1)
	}
	return source
}

func (view *TemplateSet) parse(group int) (*template.Template, error) {
	tag := view.catalog.state().groupTags[group]
	source := strings.Replace(view.catalog.translateBody(view.source(), tag), `<html lang="en"`, `<html lang="`+slotMarker(slotLanguage)+`" dir="`+slotMarker(slotDirection)+`"`, 1)
	slots := template.FuncMap{slotPicker: slotFunc(slotPicker)}
	for name := range view.runtime.RequestHTML {
		slots[name] = slotFunc(name)
	}
	catalog := view.catalog
	page, err := template.New(view.name).Funcs(template.FuncMap{
		"t": func(value string) string { return catalog.Localize(tag, value) },
	}).Funcs(view.parseFuncs).Funcs(slots).Parse(source)
	if err != nil {
		slog.Error("localized template failed to parse", "template", view.name, "language", tag, "error", err)
		return nil, fmt.Errorf("parse %s template for %s: %w", view.name, tag, err)
	}
	return page, nil
}

func (view *TemplateSet) template(tag string) (*template.Template, error) {
	group, _, ok := view.catalog.group(tag)
	if !ok {
		return nil, fmt.Errorf("render %s template: unsupported language %q", view.name, tag)
	}
	return view.views()[group]()
}

func (view *TemplateSet) Prepare(tag string) error {
	_, err := view.template(tag)
	return err
}

func (view *TemplateSet) Validate() error {
	for _, supported := range view.catalog.languages {
		page, err := view.template(supported.Tag)
		if err != nil {
			return err
		}
		var escape *template.Error
		if err := page.Execute(io.Discard, nil); errors.As(err, &escape) {
			return fmt.Errorf("escape %s template for %s: %w", view.name, supported.Tag, err)
		}
	}
	return nil
}

func (view *TemplateSet) Execute(writer http.ResponseWriter, request *http.Request, data any) error {
	return view.render(writer, request, "", data)
}

func (view *TemplateSet) ExecuteTemplate(writer http.ResponseWriter, request *http.Request, name string, data any) error {
	return view.render(writer, request, name, data)
}

func (view *TemplateSet) render(writer http.ResponseWriter, request *http.Request, name string, data any) error {
	runtime := view.runtime
	tag, preference := runtime.PreferredLanguage(request), runtime.LanguagePreference(request)
	runtime.SetLanguageHeaders(writer, tag)
	if selected, ok := runtime.MatchLanguage(request.URL.Query().Get("lang")); ok {
		preference = selected
	}
	page, err := view.template(tag)
	if err != nil {
		return err
	}
	rendered := borrowBuffer()
	defer returnBuffer(rendered)
	if name != "" {
		err = page.ExecuteTemplate(rendered, name, data)
	} else {
		err = page.Execute(rendered, data)
	}
	if err != nil {
		return err
	}
	output := borrowBuffer()
	defer returnBuffer(output)
	fillSlots(output, rendered.Bytes(), slotValues{view: view, request: request, tag: tag, preference: preference})
	_, err = writer.Write(output.Bytes())
	return err
}
