package dashboard

import (
	"testing"
)

func TestCoverageUpdateFailuresPreserveBoard(t *testing.T) {
	for _, mode := range []string{"empty", "version", "missing", "invalid", "duplicate", "storage"} {
		t.Run(mode, func(t *testing.T) {
			app := testApp(1, "https://one.test", false)
			service, store := serviceForTest(t, testBoard(app, testApp(2, "https://two.test", false)))
			name, address := "New", "https://new.test"
			input := UpdateInput{Name: &name, URL: &address, ExpectedVersion: 1}
			id := app.ID
			configureCoverageUpdateFailure(mode, &input, store, &id)
			before := service.BoardExport()
			if _, _, err := service.Update(t.Context(), id, input, "Owner"); err == nil {
				t.Fatal("invalid update accepted")
			}
			assertCoverageBoardUnchanged(t, service, before)
			if mode != "storage" && store.saveCount != 0 {
				t.Fatal("rejected update saved")
			}
		})
	}
	service, store := serviceForTest(t, testBoard())
	if _, err := service.Reorder(t.Context(), nil, 2, "Owner"); err == nil || store.saveCount != 0 {
		t.Fatal("stale reorder accepted")
	}
}

func TestCoverageExternalImportRejectsOnlyInvalidCandidates(t *testing.T) {
	service, store := serviceForTest(t, testBoard())
	if _, err := service.ImportExternal(t.Context(), ExternalImport{Source: "homarr", Content: `{"apps":[{"name":"Media","url":"ftp://invalid.test"}]}`}, 1, "Owner"); err == nil || store.saveCount != 0 {
		t.Fatal("invalid candidates imported")
	}
}

func configureCoverageUpdateFailure(mode string, input *UpdateInput, store *memoryStore, id *string) {
	switch mode {
	case "empty":
		*input = UpdateInput{}
	case "version":
		input.ExpectedVersion = 2
	case "missing":
		*id = testApp(3, "", false).ID
	case "invalid":
		*input.Name = ""
	case "duplicate":
		*input.URL = "https://two.test"
	case "storage":
		store.failSave = true
	}
}
