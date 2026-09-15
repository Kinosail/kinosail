package updatecontrol

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStorePersistenceAndTransitionRemainingEdges(t *testing.T) { //nolint:cyclop // One failure matrix proves every store operation remains atomic.
	t.Parallel()
	db := newMemoryDocumentStore()
	store, err := New(db, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	db.loadErr = errors.New("load failed")
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

func TestHTTPAndReleaseRemainingEdges(t *testing.T) { //nolint:cyclop // One adapter matrix covers independent presentation failures.
	t.Parallel()
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

	if _, err := (Manifest{}).Select("plan9", "mips"); err == nil {
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

func TestPersistedStateRemainingValidationEdges(t *testing.T) {
	t.Parallel()
	valid := State{SchemaVersion: 1}
	for name, state := range map[string]State{
		"schema":       {},
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
