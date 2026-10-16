package localization

import (
	"html/template"
	"strings"
)

type stringWriter interface {
	WriteString(string) (int, error)
}

type pickerOptions struct {
	markup  string
	selects map[string]int
}

func newPickerOptions(languages []Language) pickerOptions {
	var markup strings.Builder
	selects := make(map[string]int, len(languages))
	for _, supported := range languages {
		markup.WriteString(`<option value="` + template.HTMLEscapeString(supported.Tag) + `"`)
		if _, exists := selects[supported.Tag]; !exists {
			selects[supported.Tag] = markup.Len()
		}
		markup.WriteString(`>` + template.HTMLEscapeString(supported.Name) + `</option>`)
	}
	return pickerOptions{markup: markup.String(), selects: selects}
}

func (catalog *Catalog) PickerHTML(tag, preference, csrf string, compact bool) template.HTML {
	var picker strings.Builder
	catalog.writePicker(&picker, tag, preference, csrf, compact)
	return template.HTML(picker.String()) //nolint:gosec // Every dynamic value is escaped by writePicker.
}

func (catalog *Catalog) writePicker(picker stringWriter, tag, preference, csrf string, compact bool) {
	label := template.HTMLEscapeString(catalog.Localize(tag, "Language"))
	writeStrings(picker, `<form class="language-picker" action="/language" method="post" aria-label="`, label, `">`)
	if csrf != "" {
		writeStrings(picker, `<input type="hidden" name="_csrf" value="`, template.HTMLEscapeString(csrf), `">`)
	}
	writeStrings(picker, `<label>`, label, `<select name="language" aria-label="`, label, `"><option value="auto"`)
	if preference == "auto" {
		writeStrings(picker, ` selected`)
	}
	writeStrings(picker, `>`, template.HTMLEscapeString(catalog.Localize(tag, "Automatic (browser)")), `</option>`)
	options := catalog.options()
	if offset, ok := options.selects[preference]; ok {
		writeStrings(picker, options.markup[:offset], ` selected`, options.markup[offset:])
	} else {
		writeStrings(picker, options.markup)
	}
	writeStrings(picker, `</select></label><button class="quiet">`, template.HTMLEscapeString(catalog.Localize(tag, "Save")), `</button>`)
	if compact {
		writeStrings(picker, `<a class="mode" href="/language">`, label, `</a>`)
	}
	writeStrings(picker, `</form>`)
}

func writeStrings(writer stringWriter, values ...string) {
	for _, value := range values {
		_, _ = writer.WriteString(value)
	}
}
