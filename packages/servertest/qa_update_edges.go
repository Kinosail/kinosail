package servertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

// UpdateSettings binds the app's persisted update preference.
type UpdateSettings struct {
	Automatic     func() bool
	SaveAutomatic func(bool) error
}

// AssertManualUpdateCheckValidatesEmptyFormAndRedirects preserves the Player regression against the supplied app bindings.
func AssertManualUpdateCheckValidatesEmptyFormAndRedirects(t *testing.T, settings UpdateSettings, check func(*updatecontrol.Checker, string) http.HandlerFunc) {
	checker, err := updatecontrol.NewChecker(updatecontrol.CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: settings.Automatic, SaveAutomatic: settings.SaveAutomatic,
		Source: updatecontrol.ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			return "v1.0.0", "", false, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/updates?unexpected=true", strings.NewReader(""))
	invalid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	check(checker, "/settings#updates")(response, invalid)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid update check = %d", response.Code)
	}
	valid := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/updates", strings.NewReader(""))
	valid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	check(checker, "/settings#updates")(response, valid)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings#updates" {
		t.Fatalf("valid update check = %d location=%q", response.Code, response.Header().Get("Location"))
	}
}

// AssertUpdateAdaptersPersistPreference preserves the Player regression against the supplied app bindings.
func AssertUpdateAdaptersPersistPreference(t *testing.T, app string, newSettings func(string) UpdateSettings, policy updatecontrol.Policy, stateSchema, configurationSchema int) {
	dataDir := t.TempDir()
	settings := newSettings(dataDir)
	if err := settings.SaveAutomatic(false); err != nil {
		t.Fatal(err)
	}
	if reloaded := newSettings(dataDir); reloaded.Automatic() {
		t.Fatal("manual update preference was not persisted")
	}
	manager, err := updatecontrol.New(nil, policy)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := manager.Plan("v1.0.0", false)
	if err != nil || plan.StateSchema != stateSchema || plan.ConfigurationSchema != configurationSchema {
		t.Fatalf("%s update schema = %#v, err=%v", app, plan, err)
	}
}
