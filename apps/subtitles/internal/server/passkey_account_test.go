package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasskeyAccountRendersHonestUsageStatesAndCompactRemoval(t *testing.T) {
	t.Parallel()
	body := httptest.NewRecorder()
	data := struct {
		Passkeys         []passkeySummary
		HasPasskeys      bool
		Next, AccountURL string
	}{[]passkeySummary{{Display: "tracked-key", UsageTracked: true, LastUsedLabel: "Aug 27, 2026 at 10:42 AM", MostRecent: true, BackupEligible: true, BackedUp: true}, {Display: "ready-key", UsageTracked: true, BackupEligible: true}, {Display: "device-key", UsageTracked: true}, {Display: "warning-key", UsageTracked: true, CloneWarning: true}, {Display: "legacy-key"}}, true, "/", "http://localhost:38128/account"}
	if err := accountView.Execute(body, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/account", nil), data); err != nil {
		t.Fatal(err)
	}
	page := body.Body.String()
	for _, want := range []string{"Ways to sign in", "Last used Aug 27, 2026 at 10:42 AM", "Most recently used", "Last use is unknown", "Available on your synced devices", "Can sync across your devices", "Saved only on this device", "This passkey may have been copied.", "Extra sign-in protection", "Set up an authenticator app", "Organization sign-in", `class="passkey-list"`, `class="danger"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("account page does not contain %q: %s", want, page)
		}
	}
	for _, unwanted := range []string{"use counter", "Usage not recorded", "Synced credential", "Backup-capable credential", "Device-bound credential", "authenticator counter anomaly", "Single sign-on"} {
		if strings.Contains(page, unwanted) {
			t.Fatalf("account page contains technical passkey wording %q: %s", unwanted, page)
		}
	}
}
