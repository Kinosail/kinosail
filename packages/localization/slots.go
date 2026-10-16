package localization

import (
	"bytes"
	"crypto/rand"
	"html/template"
	"net/http"
	"sync"
)

const (
	slotLanguage  = "lang"
	slotDirection = "dir"
	slotPicker    = "languagePicker"
	maxPooledPage = 1 << 20
)

var slotPrefix = sync.OnceValue(func() string { return "\x00" + rand.Text() + ":" })

var pageBuffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func slotMarker(name string) string {
	return slotPrefix() + name + "\x00"
}

func slotFunc(name string) func() template.HTML {
	marker := template.HTML(slotMarker(name)) //nolint:gosec // The marker is replaced before any byte reaches the response.
	return func() template.HTML { return marker }
}

func borrowBuffer() *bytes.Buffer {
	buffer, _ := pageBuffers.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func returnBuffer(buffer *bytes.Buffer) {
	if buffer.Cap() <= maxPooledPage {
		pageBuffers.Put(buffer)
	}
}

type slotValues struct {
	view       *TemplateSet
	request    *http.Request
	tag        string
	preference string
}

func fillSlots(output *bytes.Buffer, page []byte, values slotValues) {
	prefix := slotPrefix()
	output.Grow(len(page) + 8<<10)
	for {
		start := bytes.Index(page, []byte(prefix))
		if start < 0 {
			output.Write(page)
			return
		}
		end := bytes.IndexByte(page[start+len(prefix):], 0)
		if end < 0 {
			output.Write(page)
			return
		}
		output.Write(page[:start])
		values.write(output, page[start+len(prefix):start+len(prefix)+end])
		page = page[start+len(prefix)+end+1:]
	}
}

func (values slotValues) write(output *bytes.Buffer, name []byte) {
	switch string(name) {
	case slotLanguage:
		output.WriteString(values.tag)
	case slotDirection:
		output.WriteString(values.view.catalog.Direction(values.tag))
	case slotPicker:
		runtime := values.view.runtime
		values.view.catalog.writePicker(output, values.tag, values.preference, runtime.CSRFForRequest(values.request), values.request.URL.Path == "/")
	default:
		if html := values.view.runtime.RequestHTML[string(name)]; html != nil {
			output.WriteString(string(html(values.request)))
		}
	}
}
