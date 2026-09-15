package updatecontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type commandMemoryDatabase struct {
	*memoryDocumentStore
	closes     int
	closeErr   error
	failLoadAt int
}

func (database *commandMemoryDatabase) LoadJSON(name string, target any) (bool, error) {
	if database.failLoadAt > 0 && database.loads+1 == database.failLoadAt {
		database.loads++
		return false, errors.New("load failed")
	}
	return database.memoryDocumentStore.LoadJSON(name, target)
}

func (database *commandMemoryDatabase) Close() error {
	database.closes++
	return database.closeErr
}

func TestBoundCommandRunsPlayerUpdateOperations(t *testing.T) { //nolint:cyclop // One command contract remains below the repository complexity ceiling.
	database := &commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore()}
	database.documents["settings.json"] = []byte(`{"updateChecks":true}`)
	command := BindCommand(CommandConfig{
		Policy: PlayerPolicy(1, 1),
		OpenDatabase: func(path string) (CommandDatabase, error) {
			if path != "/data" {
				t.Fatalf("data path = %q", path)
			}
			return database, nil
		},
		Version: func() string { return "dev" },
	})
	var output bytes.Buffer
	if err := command(CommandArtifact, strings.NewReader(validManifest), &output, "", "darwin", "arm64"); err != nil || !strings.Contains(output.String(), `"file":"kinosail-core-v1.2.3-macos-arm64.tar.gz"`) {
		t.Fatalf("artifact = %q, %v", output.String(), err)
	}
	output.Reset()
	if err := command(CommandPlan, nil, &output, "/data", "", ""); err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil || !plan.Automatic || plan.CurrentVersion != "dev" {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	report := `{"adapter":"linux","state":"current","currentVersion":"dev","checkedAt":"2026-08-27T12:00:00Z"}`
	if err := command(CommandReport, strings.NewReader(report), nil, "/data", "", ""); err != nil {
		t.Fatal(err)
	}
	if database.closes != 2 || len(database.documents[document]) == 0 {
		t.Fatalf("database closes = %d, documents = %#v", database.closes, database.documents)
	}
}

func TestBoundCommandRejectsUntrustedInputBeforeStorage(t *testing.T) { //nolint:cyclop // One table covers the complete command trust boundary.
	opens := 0
	command := BindCommand(CommandConfig{
		Policy: PlayerPolicy(1, 1),
		OpenDatabase: func(string) (CommandDatabase, error) {
			opens++
			return &commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore()}, nil
		},
		Version: func() string { return "dev" },
	})
	badManifest := strings.Replace(validManifest, `"stateSchema":1,"minimumStateSchema":1`, `"stateSchema":2,"minimumStateSchema":2`, 1)
	for name, run := range map[string]func() error{
		"unknown command":  func() error { return command("update", nil, nil, "", "", "") },
		"missing manifest": func() error { return command(CommandArtifact, nil, io.Discard, "", "linux", "amd64") },
		"missing output": func() error {
			return command(CommandArtifact, strings.NewReader(validManifest), nil, "", "linux", "amd64")
		},
		"malformed manifest": func() error {
			return command(CommandArtifact, strings.NewReader("{"), io.Discard, "", "linux", "amd64")
		},
		"incompatible manifest": func() error {
			return command(CommandArtifact, strings.NewReader(badManifest), io.Discard, "", "linux", "amd64")
		},
		"unsupported platform": func() error {
			return command(CommandArtifact, strings.NewReader(validManifest), io.Discard, "", "plan9", "amd64")
		},
		"missing plan output": func() error { return command(CommandPlan, nil, nil, "/data", "", "") },
		"missing report":      func() error { return command(CommandReport, nil, nil, "/data", "", "") },
		"malformed report":    func() error { return command(CommandReport, strings.NewReader("{"), nil, "/data", "", "") },
		"unknown report field": func() error {
			return command(CommandReport, strings.NewReader(`{"adapter":"linux","state":"current","checkedAt":"2026-08-27T12:00:00Z","extra":true}`), nil, "/data", "", "")
		},
		"trailing report": func() error {
			return command(CommandReport, strings.NewReader(`{"adapter":"linux","state":"current","checkedAt":"2026-08-27T12:00:00Z"}{}`), nil, "/data", "", "")
		},
		"oversized report": func() error {
			return command(CommandReport, strings.NewReader(strings.Repeat("x", reportLimit+1)), nil, "/data", "", "")
		},
		"unreadable report": func() error { return command(CommandReport, errorReader{}, nil, "/data", "", "") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatal("invalid command input was accepted")
			}
		})
	}
	if opens != 0 {
		t.Fatalf("rejected input opened storage %d times", opens)
	}
}

func TestBoundCommandPropagatesStorageAndOutputFailures(t *testing.T) { //nolint:cyclop,gocognit // One table covers storage lifecycle failures.
	openFailure := errors.New("open failed")
	for name, config := range map[string]CommandConfig{
		"open error":   {Policy: PlayerPolicy(1, 1), OpenDatabase: func(string) (CommandDatabase, error) { return nil, openFailure }, Version: func() string { return "dev" }},
		"nil database": {Policy: PlayerPolicy(1, 1), OpenDatabase: func(string) (CommandDatabase, error) { return nil, nil }, Version: func() string { return "dev" }},
	} {
		if err := BindCommand(config)(CommandPlan, nil, io.Discard, "/data", "", ""); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if err := BindCommand(CommandConfig{Policy: PlayerPolicy(1, 1), OpenDatabase: func(string) (CommandDatabase, error) { return nil, openFailure }, Version: func() string { return "dev" }})(CommandReport, strings.NewReader(`{"adapter":"linux","state":"current","currentVersion":"dev","checkedAt":"2026-08-27T12:00:00Z"}`), nil, "/data", "", ""); err == nil {
		t.Fatal("report storage failure was accepted")
	}
	loadFailure := &commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore(), failLoadAt: 1}
	command := testUpdateCommand(loadFailure)
	if err := command(CommandPlan, nil, io.Discard, "/data", "", ""); err == nil || loadFailure.closes != 1 {
		t.Fatalf("initial load failure = %v, closes %d", err, loadFailure.closes)
	}
	settingsFailure := &commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore(), failLoadAt: 2}
	if err := testUpdateCommand(settingsFailure)(CommandPlan, nil, io.Discard, "/data", "", ""); err == nil || settingsFailure.closes != 1 {
		t.Fatalf("settings load failure = %v, closes %d", err, settingsFailure.closes)
	}
	planFailure := &commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore(), failLoadAt: 3}
	if err := testUpdateCommand(planFailure)(CommandPlan, nil, io.Discard, "/data", "", ""); err == nil || planFailure.closes != 1 {
		t.Fatalf("plan load failure = %v, closes %d", err, planFailure.closes)
	}
	for _, name := range []string{CommandArtifact, CommandPlan} {
		input := io.Reader(nil)
		dataDir := "/data"
		if name == CommandArtifact {
			input, dataDir = strings.NewReader(validManifest), ""
		}
		if err := testUpdateCommand(&commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore()})(name, input, errorWriter{}, dataDir, "linux", "amd64"); err == nil {
			t.Fatalf("%s output failure was accepted", name)
		}
	}
	if err := testUpdateCommand(&commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore()})(CommandReport, strings.NewReader(`{}`), nil, "/data", "", ""); err == nil {
		t.Fatal("invalid semantic report was accepted")
	}
	if err := testUpdateCommand(&commandMemoryDatabase{memoryDocumentStore: newMemoryDocumentStore()})(CommandPlan, nil, io.Discard, "", "", ""); err == nil {
		t.Fatal("empty storage path was accepted")
	}
}

func TestBindCommandRejectsInvalidConfiguration(t *testing.T) {
	valid := CommandConfig{Policy: PlayerPolicy(1, 1), OpenDatabase: func(string) (CommandDatabase, error) { return nil, nil }, Version: func() string { return "dev" }}
	for name, mutate := range map[string]func(*CommandConfig){
		"policy":   func(config *CommandConfig) { config.Policy = Policy{} },
		"database": func(config *CommandConfig) { config.OpenDatabase = nil },
		"version":  func(config *CommandConfig) { config.Version = nil },
	} {
		config := valid
		mutate(&config)
		if err := BindCommand(config)(CommandPlan, nil, io.Discard, "/data", "", ""); err == nil {
			t.Fatalf("invalid %s configuration was accepted", name)
		}
	}
}

func TestCommandCoverageClosesStoreFailureEdges(t *testing.T) { //nolint:cyclop // One failure matrix proves every store operation remains atomic.
	database := newMemoryDocumentStore()
	store, err := New(database, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	database.loadErr = errors.New("load failed")
	if _, err = store.View("v1.0.0"); err == nil {
		t.Fatal("view accepted a load failure")
	}
	if _, err = store.Plan("v1.0.0", false); err == nil {
		t.Fatal("plan accepted a load failure")
	}
	if _, err = store.Request("v1.0.1"); err == nil {
		t.Fatal("request accepted a load failure")
	}
	report := Report{Adapter: "linux", State: "current", CurrentVersion: "v1.0.0", CheckedAt: time.Now()}
	if err = store.Report(report); err == nil {
		t.Fatal("report accepted a load failure")
	}

	store, _ = New(nil, PlayerPolicy(1, 1))
	if err = store.Report(Report{Adapter: "linux", State: "current", CurrentVersion: "v1.0.0", RequestID: strings.Repeat("a", 32), CheckedAt: time.Now()}); err == nil {
		t.Fatal("report without a pending request was accepted")
	}
	store.random = bytes.NewReader(bytes.Repeat([]byte{1}, 16))
	if _, err = store.Request("v1.0.1"); err != nil {
		t.Fatal(err)
	}
	if err = store.Report(report); err == nil {
		t.Fatal("report omitted the pending request")
	}
	if err = store.save(State{CheckedAt: "invalid"}); err == nil {
		t.Fatal("invalid state was saved")
	}

	invalid := newMemoryDocumentStore()
	invalid.documents[document] = []byte(`{"schemaVersion":1,"status":"invalid"}`)
	if err = (&Store{db: invalid}).reload(); err == nil {
		t.Fatal("invalid persisted state was loaded")
	}
}

func TestCommandCoverageClosesHTTPAndReleaseEdges(t *testing.T) { //nolint:cyclop // One adapter matrix covers independent presentation failures.
	checker := newHTTPTestChecker(t, nil, nil, nil)
	handlers := mustHTTPHandlers(t, checker)
	response := httptest.NewRecorder()
	handlers.View(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/updates", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("view status = %d", response.Code)
	}

	checker = newHTTPTestChecker(t, nil, func(bool) error { return errors.New("save failed") }, nil)
	response = httptest.NewRecorder()
	mustHTTPHandlers(t, checker).SavePreference("/settings")(response, formRequest(t, "/settings/updates", "mode=automatic"))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("preference failure status = %d", response.Code)
	}

	checker = newHTTPTestChecker(t, nil, nil, nil)
	manager, err := New(nil, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	manager.random = errorReader{}
	checker.manager = manager
	checker.source = ReleaseSourceFunc(func(_ context.Context, _ string) (string, string, bool, error) {
		return "v1.1.0", "", false, nil
	})
	response = httptest.NewRecorder()
	mustHTTPHandlers(t, checker).CheckAndRequest("/settings")(response, formRequest(t, "/settings/updates/check", ""))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("check-and-request failure status = %d", response.Code)
	}

	if _, _, _, err = (githubReleaseSource{}).Latest(nil, ""); err == nil { //nolint:staticcheck // The nil-context trust boundary is intentional.
		t.Fatal("nil release context was accepted")
	}
	if _, err = (Manifest{}).Select("plan9", "mips"); err == nil {
		t.Fatal("unsupported release platform was selected")
	}
	manifest, signature := releaseManifestURLs("invalid", PlayerPolicy(1, 1))
	if manifest != "" || signature != "" {
		t.Fatalf("invalid release URLs = %q, %q", manifest, signature)
	}
	if _, valid := releaseVersion("18446744073709551616.0.0"); valid {
		t.Fatal("overflowing release version was accepted")
	}
}

func TestCommandCoverageClosesStateValidationEdges(t *testing.T) {
	valid := State{SchemaVersion: 1}
	for name, state := range map[string]State{
		"schema":       {},
		"request ID":   {SchemaVersion: 1, RequestID: "invalid"},
		"request":      {SchemaVersion: 1, RequestedAt: "orphan"},
		"checked time": {SchemaVersion: 1, CheckedAt: "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateState(state); err == nil {
				t.Fatalf("invalid state accepted: %#v", state)
			}
		})
	}
	if err := validateState(valid); err != nil {
		t.Fatalf("valid state = %v", err)
	}
}

func testUpdateCommand(database CommandDatabase) Command {
	return BindCommand(CommandConfig{Policy: PlayerPolicy(1, 1), OpenDatabase: func(string) (CommandDatabase, error) { return database, nil }, Version: func() string { return "dev" }})
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
