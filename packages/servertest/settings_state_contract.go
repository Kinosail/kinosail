package servertest

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/MikeO7/kinosail/packages/supporter"
)

// SettingsStateReferences points to actual field slots, never cloned storage.
type SettingsStateReferences struct {
	Libraries, AutoSkip, Navigation *[]string
	Supporter                       **supporter.State
}

// SettingsReferences assembles a view of real field slots without copying their values.
func SettingsReferences(libraries, autoSkip, navigation *[]string, state **supporter.State) SettingsStateReferences {
	return SettingsStateReferences{Libraries: libraries, AutoSkip: autoSkip, Navigation: navigation, Supporter: state}
}

// SettingsEnabler is the real integration mutation exercised by the contract.
type SettingsEnabler interface{ SetEnabled(bool) error }

// SettingsStateFixture retains actual settings references and app operations.
type SettingsStateFixture struct {
	current        SettingsStateReferences
	snapshot       func() SettingsStateReferences
	detail         func() any
	file           *string
	active         *bool
	persist        *func(string, any) error
	newIntegration func(io.Reader) (SettingsEnabler, error)
	enabled        func() bool
}

// NewSettingsStateFixture binds a real private store without replacing or cloning it.
func NewSettingsStateFixture[S any](current *S, snapshot func() S, references func(*S) SettingsStateReferences, file *string, active *bool, persist *func(string, any) error, newIntegration func(io.Reader) (SettingsEnabler, error), enabled func() bool) SettingsStateFixture {
	return SettingsStateFixture{
		current: references(current), snapshot: func() SettingsStateReferences { value := snapshot(); return references(&value) }, detail: func() any { return *current },
		file: file, active: active, persist: persist, newIntegration: newIntegration, enabled: enabled,
	}
}

// SettingsStateContracts runs both original contracts against independently constructed stores.
func SettingsStateContracts(t *testing.T, newFixture func() SettingsStateFixture) {
	t.Helper()
	t.Run("TestSettingsSnapshotDoesNotAliasStoredState", func(t *testing.T) { assertSettingsSnapshotIsolation(t, newFixture()) })
	t.Run("TestHomeAssistantSettingFailureHasNoStateSideEffect", func(t *testing.T) { assertHomeAssistantPersistenceFailure(t, newFixture()) })
}

func assertSettingsSnapshotIsolation(t *testing.T, fixture SettingsStateFixture) {
	t.Helper()
	stored := fixture.current
	*stored.Libraries = []string{"Movies"}
	*stored.AutoSkip = []string{"intro"}
	*stored.Navigation = []string{"home"}
	*stored.Supporter = &supporter.State{InstallationKey: "installation"}
	snapshot := fixture.snapshot()
	(*snapshot.Libraries)[0] = "Changed"
	(*snapshot.AutoSkip)[0] = "credits"
	(*snapshot.Navigation)[0] = "movies"
	(*snapshot.Supporter).InstallationKey = "changed"
	if (*stored.Libraries)[0] != "Movies" || (*stored.AutoSkip)[0] != "intro" || (*stored.Navigation)[0] != "home" || (*stored.Supporter).InstallationKey != "installation" {
		t.Fatalf("settings snapshot modified stored state: %+v", fixture.detail())
	}
}

func assertHomeAssistantPersistenceFailure(t *testing.T, fixture SettingsStateFixture) {
	t.Helper()
	*fixture.file = "settings.json"
	*fixture.active = true
	*fixture.persist = func(string, any) error { return errors.New("forced persistence failure") }
	integration, err := fixture.newIntegration(bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if err = integration.SetEnabled(false); err == nil {
		t.Fatal("Home Assistant setting succeeded after persistence failed")
	}
	if !fixture.enabled() {
		t.Fatal("failed Home Assistant setting changed active state")
	}
}
