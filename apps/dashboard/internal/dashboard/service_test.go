package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu        sync.Mutex
	data      []byte
	saveCount int
	failSave  bool
}

func (store *memoryStore) LoadJSON(_ context.Context, _ string, target any) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.data == nil {
		return false, nil
	}
	return true, json.Unmarshal(store.data, target)
}

func (store *memoryStore) SaveJSON(_ context.Context, _ string, value any) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.saveCount++
	if store.failSave {
		return errors.New("save failed")
	}
	data, err := json.Marshal(value)
	if err == nil {
		store.data = data
	}
	return err
}

func serviceForTest(t *testing.T, board Board) (*Service, *memoryStore) {
	t.Helper()
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{data: data}
	service, err := NewService(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func testBoard(apps ...App) Board {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	return Board{Title: "Home", Version: 1, Apps: apps, Audit: []AuditEvent{}, UpdatedAt: now}
}

func testApp(index int, address string, checkEnabled bool) App {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	return App{
		ID:           fmt.Sprintf("%024d", index),
		Name:         fmt.Sprintf("Application %d", index),
		URL:          address,
		HealthURL:    address,
		CheckEnabled: checkEnabled,
		Icon:         "app",
		Accent:       "slate",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestNormalizeCreateRejectsInvalidInput(t *testing.T) {
	valid := CreateInput{Name: "Media", URL: "https://media.example.test", Icon: "app", Accent: "slate"}
	tests := map[string]func(CreateInput) CreateInput{
		"missing name":        func(input CreateInput) CreateInput { input.Name = " "; return input },
		"unsupported catalog": func(input CreateInput) CreateInput { input.CatalogID = "unknown"; return input },
		"unsupported scheme":  func(input CreateInput) CreateInput { input.URL = "ftp://media.example.test"; return input },
		"credentials": func(input CreateInput) CreateInput {
			input.URL = "https://owner:secret@media.example.test"
			return input
		},
		"query":           func(input CreateInput) CreateInput { input.URL += "?token=secret"; return input },
		"fragment":        func(input CreateInput) CreateInput { input.URL += "#private"; return input },
		"encoded control": func(input CreateInput) CreateInput { input.URL += "/%0Aadmin"; return input },
		"oversized URL": func(input CreateInput) CreateInput {
			input.URL = "https://example.test/" + strings.Repeat("a", 2048)
			return input
		},
		"oversized description": func(input CreateInput) CreateInput { input.Description = strings.Repeat("a", 121); return input },
		"invalid icon":          func(input CreateInput) CreateInput { input.Icon = "not an icon"; return input },
		"invalid accent":        func(input CreateInput) CreateInput { input.Accent = "magenta"; return input },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeCreate(mutate(valid)); err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
		})
	}
}

func TestNormalizeCreateCanonicalizesURL(t *testing.T) {
	app, err := normalizeCreate(CreateInput{Name: "Media", URL: "HTTPS://MEDIA.EXAMPLE.TEST:443/library", Icon: "app", Accent: "slate"})
	if err != nil {
		t.Fatal(err)
	}
	if app.URL != "https://media.example.test/library" || app.HealthURL != app.URL {
		t.Fatalf("unexpected normalized addresses: URL=%q health=%q", app.URL, app.HealthURL)
	}
}

func TestMutationRequiresCurrentVersionWithoutSideEffects(t *testing.T) {
	service, store := serviceForTest(t, testBoard())
	input := CreateInput{Name: "Media", URL: "https://media.example.test", Icon: "app", Accent: "slate"}

	if _, _, err := service.Create(context.Background(), input, "Owner"); err == nil || err.Error() != "expectedVersion is required" {
		t.Fatalf("expected required version error, got %v", err)
	}
	input.ExpectedVersion = 2
	if _, _, err := service.Create(context.Background(), input, "Owner"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if snapshot := service.Snapshot(); snapshot.Version != 1 || len(snapshot.Apps) != 0 {
		t.Fatalf("rejected changes mutated board: %+v", snapshot)
	}
	if store.saveCount != 0 {
		t.Fatalf("rejected changes saved state %d times", store.saveCount)
	}
}

func TestReorderRequiresOneExactPermutation(t *testing.T) {
	apps := []App{
		testApp(1, "https://one.example.test", false),
		testApp(2, "https://two.example.test", false),
		testApp(3, "https://three.example.test", false),
	}
	service, store := serviceForTest(t, testBoard(apps...))
	initial := []string{apps[0].ID, apps[1].ID, apps[2].ID}
	tests := map[string][]string{
		"missing":  initial[:2],
		"repeated": {apps[0].ID, apps[0].ID, apps[2].ID},
		"unknown":  {apps[0].ID, apps[1].ID, "999999999999999999999999"},
	}
	for name, order := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Reorder(context.Background(), order, 1, "Owner"); err == nil {
				t.Fatal("expected invalid order to be rejected")
			}
			if got := appIDs(service.Snapshot().Apps); !reflect.DeepEqual(got, initial) {
				t.Fatalf("rejected order changed board: %v", got)
			}
		})
	}
	if store.saveCount != 0 {
		t.Fatalf("invalid orders saved state %d times", store.saveCount)
	}

	wanted := []string{apps[2].ID, apps[0].ID, apps[1].ID}
	receipt, err := service.Reorder(context.Background(), wanted, 1, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if got := appIDs(service.Snapshot().Apps); !reflect.DeepEqual(got, wanted) {
		t.Fatalf("got order %v, want %v", got, wanted)
	}
	if receipt.BeforeVersion != 1 || receipt.AfterVersion != 2 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}

	assertReorderPersistenceRollback(t, service, store, initial, wanted)
}

func assertReorderPersistenceRollback(t *testing.T, service *Service, store *memoryStore, initial, wanted []string) {
	t.Helper()
	store.failSave = true
	if _, err := service.Reorder(context.Background(), initial, 2, "Owner"); !errors.Is(err, ErrState) {
		t.Fatalf("expected state failure, got %v", err)
	}
	if got := appIDs(service.Snapshot().Apps); !reflect.DeepEqual(got, wanted) || service.Snapshot().Version != 2 {
		t.Fatalf("failed save did not roll back: order=%v version=%d", got, service.Snapshot().Version)
	}
}

func TestRestoreRejectsURLConflictAndFullBoard(t *testing.T) {
	conflicting := testApp(2, "https://same.example.test", false)
	board := testBoard(testApp(1, "https://same.example.test", false))
	board.Removed = []RemovedApp{{App: conflicting, Position: 0, RemovedAt: board.UpdatedAt}}
	service, store := serviceForTest(t, board)
	if _, _, err := service.Restore(context.Background(), conflicting.ID, 1, "Owner"); err == nil {
		t.Fatal("expected URL conflict")
	}
	if store.saveCount != 0 || len(service.Snapshot().Removed) != 1 {
		t.Fatal("conflicting restore caused side effects")
	}

	apps := make([]App, 0, MaxApps)
	for index := 1; index <= MaxApps; index++ {
		apps = append(apps, testApp(index, fmt.Sprintf("https://app-%d.example.test", index), false))
	}
	removed := testApp(MaxApps+1, "https://removed.example.test", false)
	full := testBoard(apps...)
	full.Removed = []RemovedApp{{App: removed, Position: 0, RemovedAt: full.UpdatedAt}}
	service, store = serviceForTest(t, full)
	if _, _, err := service.Restore(context.Background(), removed.ID, 1, "Owner"); err == nil {
		t.Fatal("expected full-board restore rejection")
	}
	if store.saveCount != 0 || len(service.Snapshot().Apps) != MaxApps || len(service.Snapshot().Removed) != 1 {
		t.Fatal("full-board restore caused side effects")
	}
}

func TestBoardExportImportsConfigurationRoundTrip(t *testing.T) {
	apps := []App{
		testApp(1, "https://one.example.test", true),
		testApp(2, "http://two.example.test:8080", false),
	}
	apps[0].Description, apps[0].Category, apps[0].Favorite = "Local media", "Media", true
	sourceBoard := testBoard(apps...)
	sourceBoard.Title = "Household"
	source, _ := serviceForTest(t, sourceBoard)
	exported := source.BoardExport()

	target, _ := serviceForTest(t, testBoard())
	receipt, err := target.ImportBoard(context.Background(), Import{Title: exported.Title, Apps: exported.Apps, ExpectedVersion: 1}, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	got := target.BoardExport()
	if got.Title != exported.Title || !reflect.DeepEqual(got.Apps, exported.Apps) {
		t.Fatalf("configuration changed during round trip:\ngot  %+v\nwant %+v", got, exported)
	}
	if receipt.Action != "board.imported" || receipt.BeforeVersion != 1 || receipt.AfterVersion != 2 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestNewServiceRejectsInvalidPersistedBoard(t *testing.T) {
	first := testApp(1, "https://one.example.test", false)
	duplicate := testApp(1, "https://two.example.test", false)
	board := testBoard(first, duplicate)
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(context.Background(), &memoryStore{data: data}); err == nil {
		t.Fatal("expected duplicate persisted identifier rejection")
	}
}

func TestImportRejectsInvertedOrFutureTimestampsWithoutSaving(t *testing.T) {
	service, store := serviceForTest(t, testBoard())
	for name, change := range map[string]func(*App){
		"inverted": func(app *App) { app.CreatedAt = app.UpdatedAt.Add(time.Hour) },
		"future":   func(app *App) { app.UpdatedAt = time.Now().UTC().Add(time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			app := testApp(1, "https://one.example.test", false)
			change(&app)
			if _, err := service.ImportBoard(context.Background(), Import{Title: "Home", Apps: []App{app}, ExpectedVersion: 1}, "Owner"); err == nil {
				t.Fatal("expected timestamp rejection")
			}
			if store.saveCount != 0 || service.Snapshot().Version != 1 || len(service.Snapshot().Apps) != 0 {
				t.Fatal("rejected import caused side effects")
			}
		})
	}
}

func appIDs(apps []AppView) []string {
	result := make([]string, 0, len(apps))
	for _, app := range apps {
		result = append(result, app.ID)
	}
	return result
}
