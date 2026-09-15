package markers

import (
	"errors"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestOwnerMarkerWriteFailuresAreReportedWithoutMutation(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			item := movieItems(1)[0]
			analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
			if err := analyzer.SetManual(item, "intro", 1, 10, 100); err != nil {
				t.Fatal(err)
			}
			before := maps.Clone(analyzer.records)
			analyzer.persist = func(string, any) error { return errors.New("private/storage/path") }
			mux := http.NewServeMux()
			RegisterAdmin(mux, testAdmin(analyzer, item, nil))
			path, body := "/api/v1/items/"+item.ID+"/markers", `{"type":"intro","start":2,"end":11}`
			if method == http.MethodDelete {
				path, body = path+"/intro", ""
			}
			response := markerRequest(t, mux, method, path, "application/json", body)
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private/") {
				t.Fatalf("storage failure response = %d %s", response.Code, response.Body.String())
			}
			if !reflect.DeepEqual(analyzer.records, before) {
				t.Fatal("failed Owner write changed markers")
			}
		})
	}
}
