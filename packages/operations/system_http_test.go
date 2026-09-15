package operations

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

type systemTestView func(http.ResponseWriter, *http.Request, any) error

func (view systemTestView) Execute(w http.ResponseWriter, r *http.Request, data any) error {
	return view(w, r, data)
}

func TestSystemHandlerReadsFreshSnapshotsAndPreservesRenderErrors(t *testing.T) {
	var calls []string
	source := SystemSource[string]{
		Logs: func() []auditjournal.Event {
			calls = append(calls, "logs")
			return []auditjournal.Event{{Action: "scan"}}
		},
		Playback:    func() []string { calls = append(calls, "playback"); return []string{"Arrival"} },
		Diagnostics: func() DiagnosticReport { calls = append(calls, "diagnostics"); return DiagnosticReport{Healthy: true} },
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/system", nil)
	view := systemTestView(func(w http.ResponseWriter, r *http.Request, raw any) error {
		calls = append(calls, "render")
		data := raw.(SystemPage[string])
		if r != request || w.Header().Get("Content-Type") != "text/html; charset=utf-8" || data.Logs[0].Action != "scan" || data.Playback[0] != "Arrival" || !data.Diagnostics.Healthy {
			t.Fatalf("incorrect page projection: %#v", data)
		}
		if len(calls) == 4 {
			return errors.New("render failure")
		}
		return nil
	})
	handler := NewSystemHandler(source, view)
	for range 2 {
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	want := []string{"logs", "playback", "diagnostics", "render", "logs", "playback", "diagnostics", "render"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestSystemHandlerRejectsMissingDependenciesBeforeReads(t *testing.T) {
	for _, missing := range []string{"logs", "playback", "diagnostics", "view"} {
		t.Run(missing, func(t *testing.T) {
			source := SystemSource[string]{Logs: func() []auditjournal.Event { t.Fatal("unexpected read"); return nil }, Playback: func() []string { t.Fatal("unexpected read"); return nil }, Diagnostics: func() DiagnosticReport { t.Fatal("unexpected read"); return DiagnosticReport{} }}
			var view systemView = systemTestView(func(http.ResponseWriter, *http.Request, any) error { return nil })
			switch missing {
			case "logs":
				source.Logs = nil
			case "playback":
				source.Playback = nil
			case "diagnostics":
				source.Diagnostics = nil
			case "view":
				view = nil
			}
			defer func() {
				if recover() == nil {
					t.Fatal("missing dependency accepted")
				}
			}()
			NewSystemHandler(source, view)
		})
	}
}

func TestSystemViewBrandingAndValidation(t *testing.T) {
	for _, product := range []string{"Player", "Subtitles"} {
		source := SystemViewSource(SystemBrand{product, "12345678", "0", "75"})
		for _, want := range []string{"System · Kinosail " + product, "icon.svg?v=12345678", "theme.js?v=0", "app.css?v=75", "Recent activity", "Recent playback", "Download safe diagnostics"} {
			if !strings.Contains(source, want) {
				t.Fatalf("source missing %q", want)
			}
		}
		if strings.Contains(source, "__") {
			t.Fatal("unresolved brand placeholder")
		}
	}
	for _, invalid := range []SystemBrand{{"Other", "5", "6", "75"}, {"Player", "", "6", "75"}, {"Player", "123456789", "6", "75"}, {"Player", "/", "6", "75"}, {"Player", ":", "6", "75"}, {"Player", "5", "x", "75"}, {"Player", "5", "6", "x"}} {
		t.Run(invalid.Product+invalid.IconVersion+invalid.ThemeVersion+invalid.StylesheetVersion, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid brand accepted")
				}
			}()
			SystemViewSource(invalid)
		})
	}
}
