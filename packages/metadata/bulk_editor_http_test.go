package metadata

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type bulkHTTPResult struct {
	renders int
	page    BulkPage
	fails   int
	message string
	status  int
	err     error
}

func newHTTPBulkEditor(index *bulkIndexStub, effects *bulkEffects, result *bulkHTTPResult) *BulkEditor {
	editor := effects.editor(index)
	editor.page = PlayerBulkPage()
	editor.render = func(writer http.ResponseWriter, _ *http.Request, data any) error {
		result.renders++
		result.page = data.(BulkPage)
		_, _ = writer.Write([]byte("rendered"))
		return result.err
	}
	editor.fail = func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
		result.fails++
		result.message, result.status = message, status
	}
	return editor
}

func TestBulkEditorPage(t *testing.T) {
	t.Parallel()
	index := &bulkIndexStub{snapshot: []library.Item{{ID: bulkIDOne, Title: "zulu"}, {ID: bulkIDTwo, Title: "Alpha"}, {ID: "1111111111111111", Title: "alpha"}}}
	result := &bulkHTTPResult{}
	editor := newHTTPBulkEditor(index, &bulkEffects{initial: map[string]Record{}}, result)
	recorder := httptest.NewRecorder()
	editor.Page(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata/bulk", nil))
	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
	if result.renders != 1 || result.fails != 0 || result.page.Error != "" {
		t.Fatalf("render result = %#v", result)
	}
	if got := []string{result.page.Items[0].ID, result.page.Items[1].ID, result.page.Items[2].ID}; !reflect.DeepEqual(got, []string{bulkIDTwo, "1111111111111111", bulkIDOne}) {
		t.Fatalf("sorted ids = %#v", got)
	}
	if result.page.Product != "Kinosail Player" || result.page.ThemeScript != "/static/theme.js?v=cinema-1" || result.page.Stylesheet != "/static/app.css?v=cinema-1" {
		t.Fatalf("page profile = %#v", result.page)
	}
}

func TestBulkEditorRegister(t *testing.T) {
	t.Parallel()
	index := &bulkIndexStub{}
	result := &bulkHTTPResult{}
	editor := newHTTPBulkEditor(index, &bulkEffects{initial: map[string]Record{}}, result)
	mux, wraps := http.NewServeMux(), 0
	editor.Register(mux, func(handler http.Handler) http.Handler {
		wraps++
		return handler
	})
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata/bulk", nil))
	if wraps != 2 || result.renders != 1 || recorder.Code != http.StatusOK {
		t.Fatalf("registered routes wraps=%d result=%#v status=%d", wraps, result, recorder.Code)
	}
}

func TestBulkEditorPageFailures(t *testing.T) {
	t.Parallel()
	t.Run("snapshot", func(t *testing.T) {
		index := &bulkIndexStub{snapshotErr: errors.New("scan")}
		result := &bulkHTTPResult{}
		recorder := httptest.NewRecorder()
		newHTTPBulkEditor(index, &bulkEffects{initial: map[string]Record{}}, result).Page(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata/bulk", nil))
		if result.fails != 1 || result.message != "library is unavailable" || result.status != http.StatusServiceUnavailable || result.renders != 0 {
			t.Fatalf("failure = %#v", result)
		}
	})
	t.Run("render", func(t *testing.T) {
		index := &bulkIndexStub{}
		result := &bulkHTTPResult{err: errors.New("template failed")}
		recorder := httptest.NewRecorder()
		newHTTPBulkEditor(index, &bulkEffects{initial: map[string]Record{}}, result).Page(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata/bulk", nil))
		if result.fails != 1 || result.message != "template failed" || result.status != http.StatusInternalServerError || result.renders != 1 {
			t.Fatalf("failure = %#v", result)
		}
	})
}

func TestBulkEditorEditSuccess(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: bulkIDOne, Title: "Base"}
	index := &bulkIndexStub{items: map[string]library.Item{bulkIDOne: item}}
	effects, result := &bulkEffects{initial: map[string]Record{}}, &bulkHTTPResult{}
	form := url.Values{
		"itemIds": {bulkIDOne}, "title": {" New "}, "year": {"2024"}, "rating": {"R"},
		"tagline": {"Tag"}, "genres": {"Drama"}, "plot": {"Plot"},
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/bulk", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	recorder := httptest.NewRecorder()
	newHTTPBulkEditor(index, effects, result).Edit(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/" {
		t.Fatalf("response = %d location %q", recorder.Code, recorder.Header().Get("Location"))
	}
	if result.renders != 0 || result.fails != 0 || effects.saves != 1 || index.refreshes != 1 {
		t.Fatalf("effects = %#v result = %#v index = %#v", effects, result, index)
	}
}

func TestBulkEditorEditRejectsInvalidRequestsWithoutMutation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		target      string
		contentType string
		body        string
		message     string
	}{
		{"media type", "/metadata/bulk", "application/json", `{}`, "metadata request is invalid"},
		{"query", "/metadata/bulk?title=x", "application/x-www-form-urlencoded", "title=x", "metadata request is invalid"},
		{"malformed", "/metadata/bulk", "application/x-www-form-urlencoded", "title=%zz", "metadata request is invalid"},
		{"unknown", "/metadata/bulk", "application/x-www-form-urlencoded", "other=x", "metadata request is invalid"},
		{"duplicate field", "/metadata/bulk", "application/x-www-form-urlencoded", "title=one&title=two", "metadata request is invalid"},
		{"missing selection", "/metadata/bulk", "application/x-www-form-urlencoded", "title=x", ErrBulkSelection.Error()},
		{"empty patch", "/metadata/bulk", "application/x-www-form-urlencoded", "itemIds=" + bulkIDOne, ErrBulkFields.Error()},
		{"malformed selection", "/metadata/bulk", "application/x-www-form-urlencoded", "itemIds=bad&title=x", ErrBulkSelection.Error()},
		{"oversized", "/metadata/bulk", "application/x-www-form-urlencoded", "title=" + strings.Repeat("x", bulkFormMaximum+1), "metadata request is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := &bulkIndexStub{items: map[string]library.Item{}}
			effects, result := &bulkEffects{initial: map[string]Record{}}, &bulkHTTPResult{}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.target, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			newHTTPBulkEditor(index, effects, result).Edit(httptest.NewRecorder(), request)
			if result.renders != 1 || result.page.Error != test.message || result.fails != 0 {
				t.Fatalf("render result = %#v", result)
			}
			if index.finds != 0 || effects.records != 0 || effects.saves != 0 || index.refreshes != 0 {
				t.Fatalf("invalid request effects = finds:%d records:%d saves:%d refreshes:%d", index.finds, effects.records, effects.saves, index.refreshes)
			}
		})
	}
}

func TestBulkPatchFromForm(t *testing.T) {
	t.Parallel()
	fields := []string{"title", "year", "rating", "tagline", "genres", "plot"}
	for _, field := range fields {
		t.Run("duplicate "+field, func(t *testing.T) {
			if _, err := bulkPatchFromForm(url.Values{field: {"one", "two"}}); !errors.Is(err, ErrBulkFields) {
				t.Fatalf("bulkPatchFromForm() error = %v", err)
			}
		})
	}
	blank := url.Values{}
	for _, field := range fields {
		blank[field] = []string{"  "}
	}
	if patch, err := bulkPatchFromForm(blank); err != nil || !patch.empty() {
		t.Fatalf("blank patch = %#v, %v", patch, err)
	}
}

func TestBulkMetadataHTMLUsesProductProfile(t *testing.T) {
	t.Parallel()
	view := template.Must(template.New("bulk").Funcs(template.FuncMap{"icon": func(string) string { return "icon" }}).Parse(BulkMetadataHTML))
	var output bytes.Buffer
	page := PlayerBulkPage()
	if err := view.Execute(&output, page); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, fragment := range []string{"<title>Bulk metadata · Kinosail Player</title>", `<script src="/static/theme.js?v=cinema-1"></script>`, `<link rel="stylesheet" href="/static/app.css?v=cinema-1">`} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("output missing %q", fragment)
		}
	}
	if page := SubtitlesBulkPage(); page.Product != "Kinosail Subtitles" || page.ThemeScript != "/static/theme.js?v=cinema-1" || page.Stylesheet != "/static/app.css?v=cinema-1" {
		t.Fatalf("SubtitlesBulkPage() = %#v", page)
	}
}
