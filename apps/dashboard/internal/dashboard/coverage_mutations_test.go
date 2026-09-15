package dashboard

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestCoverageServiceInitialization(t *testing.T) {
	store := &memoryStore{}
	service, err := NewService(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot(); got.Title != "Home" || got.Version != 1 || store.saveCount != 1 {
		t.Fatalf("initial board = %+v, saves %d", got, store.saveCount)
	}
}

func TestCoverageServiceInitializationFailures(t *testing.T) {
	for name, store := range map[string]*memoryStore{
		"save failure": {failSave: true}, "load failure": {data: []byte("{")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewService(t.Context(), store); err == nil {
				t.Fatal("invalid storage accepted")
			}
		})
	}
	if _, err := NewService(nil, &memoryStore{}); err == nil { //nolint:staticcheck // Verify the public nil-context rejection contract.
		t.Fatal("nil context accepted")
	}
	if _, err := NewService(t.Context(), nil); err == nil {
		t.Fatal("nil storage accepted")
	}
}

func TestCoverageCreateAndRecoveryLifecycle(t *testing.T) {
	service, _ := serviceForTest(t, testBoard())
	input := CreateInput{Name: "Media", URL: "https://media.test", ExpectedVersion: 1}
	app, created, err := service.Create(t.Context(), input, " ")
	if err != nil {
		t.Fatal(err)
	}
	if !validID(app.ID) || created.Action != "app.added" || created.AfterVersion != 2 || app.CreatedAt.IsZero() {
		t.Fatalf("created = %+v, receipt %+v", app, created)
	}
	assertCoverageRecoveryLifecycle(t, service, app)
}

func TestCoverageCreateFailuresDoNotMutate(t *testing.T) {
	for _, mode := range []string{"invalid", "duplicate", "capacity", "entropy", "storage"} {
		t.Run(mode, func(t *testing.T) {
			service, store := serviceForTest(t, testBoard(testApp(1, "https://existing.test", false)))
			input := CreateInput{Name: "Media", URL: "https://new.test", ExpectedVersion: 1}
			switch mode {
			case "invalid":
				input.Name = ""
			case "duplicate":
				input.URL = "https://existing.test"
			case "capacity":
				service.board.Apps = make([]App, MaxApps)
			case "entropy":
				service.randomID = func() (string, error) { return "", errors.New("entropy unavailable") }
			case "storage":
				store.failSave = true
			}
			before := service.BoardExport()
			if _, _, err := service.Create(t.Context(), input, "Owner"); err == nil {
				t.Fatal("create accepted failure")
			}
			assertCoverageBoardUnchanged(t, service, before)
			if mode != "storage" && store.saveCount != 0 {
				t.Fatal("rejected create persisted")
			}
		})
	}
}

func TestCoverageRecoveryFailuresDoNotMutate(t *testing.T) {
	for _, action := range []string{"remove", "restore", "rename"} {
		for _, mode := range []string{"version", "missing", "storage"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				assertCoverageRecoveryFailure(t, action, mode)
			})
		}
	}
}

func TestCoverageRemoveBoundsRecoveryList(t *testing.T) {
	board := testBoard(testApp(1, "https://one.test", false))
	for index := 2; index <= 11; index++ {
		board.Removed = append(board.Removed, RemovedApp{App: testApp(index, fmt.Sprintf("https://app%d.test", index), false), RemovedAt: board.UpdatedAt})
	}
	service, store := serviceForTest(t, board)
	if _, err := service.Remove(t.Context(), "invalid", 1, "Owner"); !errors.Is(err, ErrNotFound) || store.saveCount != 0 {
		t.Fatalf("invalid id = %v", err)
	}
	if _, err := service.Remove(t.Context(), board.Apps[0].ID, 1, "Owner"); err != nil {
		t.Fatal(err)
	}
	removed := service.Snapshot().Removed
	if len(removed) != 10 || removed[0].App.ID != board.Apps[0].ID || removed[9].App.ID != board.Removed[8].App.ID {
		t.Fatalf("bounded recovery = %+v", removed)
	}
}

func assertCoverageBoardUnchanged(t *testing.T, service *Service, want Board) {
	t.Helper()
	got := service.BoardExport()
	want = cloneBoard(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rejected mutation changed board:\ngot %+v\nwant %+v", got, want)
	}
}

func assertCoverageRecoveryFailure(t *testing.T, action, mode string) {
	t.Helper()
	app := testApp(1, "https://media.test", false)
	board := testBoard(app)
	if action == "restore" {
		board.Apps = nil
		board.Removed = []RemovedApp{{App: app, RemovedAt: board.UpdatedAt}}
	}
	service, store := serviceForTest(t, board)
	version, id, title := uint64(1), app.ID, "New"
	switch mode {
	case "version":
		version = 2
	case "missing":
		id = testApp(2, "", false).ID
		title = ""
	case "storage":
		store.failSave = true
	}
	err := coverageRecoveryAction(t, service, action, id, title, version)
	if err == nil {
		t.Fatal("mutation accepted failure")
	}
	assertCoverageBoardUnchanged(t, service, board)
	if mode != "storage" && store.saveCount != 0 {
		t.Fatal("rejected mutation persisted")
	}
}

func assertCoverageRecoveryLifecycle(t *testing.T, service *Service, app App) {
	t.Helper()
	removed, err := service.Remove(t.Context(), app.ID, 2, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot(); len(got.Apps) != 0 || len(got.Removed) != 1 || removed.AfterVersion != 3 {
		t.Fatalf("removed board = %+v", got)
	}
	assertCoverageRestoreAndRename(t, service, app)
}

func coverageRecoveryAction(t *testing.T, service *Service, action, id, title string, version uint64) error {
	t.Helper()
	var err error
	switch action {
	case "remove":
		_, err = service.Remove(t.Context(), id, version, "Owner")
	case "restore":
		_, _, err = service.Restore(t.Context(), id, version, "Owner")
	case "rename":
		_, err = service.Rename(t.Context(), title, version, "Owner")
	}
	return err
}

func assertCoverageRestoreAndRename(t *testing.T, service *Service, app App) {
	t.Helper()
	restored, receipt, err := service.Restore(t.Context(), app.ID, 3, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot(); len(got.Removed) != 0 || len(got.Apps) != 1 || restored.ID != app.ID || receipt.AfterVersion != 4 {
		t.Fatalf("restored board = %+v, app %+v", got, restored)
	}
	if receipt, err := service.Rename(t.Context(), " Household ", 4, "Owner"); err != nil || receipt.AfterVersion != 5 || service.Snapshot().Title != "Household" {
		t.Fatalf("rename = %+v, %v", receipt, err)
	}
}
