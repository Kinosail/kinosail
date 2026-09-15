package playerweb

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestCurationPagesRenderNamedSearchAndEscapedInput(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{"collection": CollectionHTML, "playlist": PlaylistHTML} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			view, err := template.New(name).Funcs(template.FuncMap{"icon": func(string) string { return "icon" }}).Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			var rendered bytes.Buffer
			if err := view.Execute(&rendered, map[string]any{"Name": "<script>title</script>", "Query": "\"<script>query</script>", "Owner": true, "Smart": false}); err != nil {
				t.Fatal(err)
			}
			html := rendered.String()
			assertCurationSearch(t, html, name)
			if strings.Contains(html, "<script>title") || strings.Contains(html, "<script>query") {
				t.Fatal("curation input was not escaped")
			}
		})
	}
}

func assertCurationSearch(t *testing.T, html, name string) {
	t.Helper()
	for _, want := range []string{`role="search" aria-label="Find something"`, `for="` + name + `-query"`, `id="` + name + `-query"`, `&lt;script&gt;title&lt;/script&gt;`, `&lt;script&gt;query&lt;/script&gt;`} {
		if !strings.Contains(html, want) {
			t.Errorf("render lacks %q", want)
		}
	}
}
