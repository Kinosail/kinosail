package server

import (
	"bytes"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func supporterUIFixture(archived bool) supporterPageData {
	collection := &supporterCollectionStatus{ID: completeFleetID, Name: "Complete Fleet", Edition: "Living", AppIDs: []string{"kino-player", supporterAppID}}
	living := supporterGrantStatus{
		Family: livingStandardFamily, Title: "Living Standard", Tier: "legacy", Name: "Legacy", Rank: 10, Active: true,
		SupportedSince: "2021-08-26T12:00:00Z", ExpiresAt: "2026-09-25T12:00:00Z", ServiceMonths: 60,
		ServiceMarks: []int{3, 6, 12, 24, 36, 60}, Founding: true, RecognitionName: "Quiet Benefactor", Collection: collection,
	}
	if archived {
		living.Tier, living.Name, living.Rank = "admiral", "Admiral", 8
		living.Active, living.Expired, living.Archived = false, true, true
		living.ExpiresAt, living.ServiceMonths, living.ServiceMarks = "2026-07-26T12:00:00Z", 24, []int{3, 6, 12, 24}
	}
	patron := supporterGrantStatus{
		Family: patronOrderFamily, Title: "Patron Order", Tier: "northstar", Name: "North Star", Rank: 9, Active: true,
		SupportedSince: "2025-08-26T12:00:00Z", Founding: true,
		Collection: &supporterCollectionStatus{ID: completeFleetID, Name: "Complete Fleet", Edition: "2026 Founders", AppIDs: []string{"kino-player", supporterAppID}},
	}
	caseStatus := supporterBadgeCase{LivingLevel: living.Rank, PatronLevel: patron.Rank, MasterworkLevel: min(living.Rank, patron.Rank), Unlocked: living.Rank + patron.Rank, Total: 20, MasterworkName: "Perfect Sync", MasterworkEarned: true, MasterworkActive: living.Active}
	masterwork := supporterBadge(livingStandardFamily, caseStatus.MasterworkLevel)
	masterwork.BadgeName = perfectSyncNames[caseStatus.MasterworkLevel-1]
	return supporterPageData{
		Status:         supporterStatus{App: supporterAppStatus{ID: supporterAppID, Name: supporterAppName}, BadgeCase: caseStatus},
		LivingStandard: supporterGrantPage(living, living.Rank, livingStandardFamily, "Monthly support · Recommended", "Living Standards are complete badges with signal geometry, service marks, and an honest active-through date.", "Choose monthly support", "https://support.example"),
		PatronOrder:    supporterGrantPage(patron, patron.Rank, patronOrderFamily, "One-time support · Permanent", "Patron Orders are permanent enamel badges. Each level has its own complete silhouette and caption emblem.", "Choose one-time support", "https://support.example"),
		Masterwork:     masterwork, MasterworkActive: living.Active, HasMasterwork: true, HasAny: true, HasAnyFleet: true,
	}
}

func TestWriteUIStateFixtures(t *testing.T) { //nolint:paralleltest,funlen // This opt-in test writes a shared browser fixture directory.
	dir := os.Getenv("KINOSAIL_UI_FIXTURE_DIR")
	if dir == "" {
		t.Skip("KINOSAIL_UI_FIXTURE_DIR is not set")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // The caller explicitly supplies this test-artifact directory.
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), "GET", "https://kinosail.test/", nil)
	localized := func(view localizedTemplate, data any) func(*bytes.Buffer) error {
		return func(output *bytes.Buffer) error {
			response := httptest.NewRecorder()
			if err := view.Execute(response, request, data); err != nil {
				return err
			}
			_, err := output.Write(response.Body.Bytes())
			return err
		}
	}
	plain := func(view *template.Template, data any) func(*bytes.Buffer) error {
		return func(output *bytes.Buffer) error { return view.Execute(output, data) }
	}
	csrf := func(view *template.Template, data any) func(*bytes.Buffer) error {
		return func(output *bytes.Buffer) error {
			response := httptest.NewRecorder()
			if err := executeCSRFTemplate(view, response, request, data); err != nil {
				return err
			}
			_, err := output.Write(response.Body.Bytes())
			return err
		}
	}

	preview := viewingImportWebPage{
		viewingImportPreview: viewingImportPreview{
			ID: "fixture", Source: "Jellyfin", ProfileName: "Living Room", ExpiresAt: time.Date(2030, 1, 2, 15, 4, 0, 0, time.UTC),
			Summary: viewingImportSummary{Activity: 2, Favorites: 1, PlaylistItems: 1, Importable: 2},
			Items:   []viewingImportItem{{SourceTitle: "Signal Garden", TargetTitle: "Signal Garden", Status: "ready", Reason: "matched by provider ID", Watched: true, Favorite: true, Playlists: []string{"Weekend"}}},
		},
		BackPath: "/settings#viewing-imports", BackLabel: "Settings", Step: "Migration preview", ApplyPath: "/settings/viewing-imports/apply", AllowSync: true,
	}
	fixtures := map[string]func(*bytes.Buffer) error{
		"api-key-created":   localized(apiKeySecretView, map[string]string{"Name": "Living Room remote", "Secret": "ks_example_local_only"}),
		"media-share-items": plain(mediaShareItemsView, []map[string]string{{"Title": "Signal Garden", "ID": "fixture"}}),
		"mfa-setup":         localized(mfaSetupView, mfaSetupPage{mfaEnrollment: mfaEnrollment{Secret: strings.Repeat("A", 32), URI: "otpauth://totp/Kinosail:Owner", RecoveryCodes: []string{"AAAA-BBBB-CCCC-DDDD", "EEEE-FFFF-GGGG-HHHH"}}}),
		"passkey-account": localized(accountView, struct {
			Passkeys         []passkeySummary
			HasPasskeys      bool
			Next, AccountURL string
		}{[]passkeySummary{{Display: "tracked-key", UsageTracked: true, LastUsedLabel: "Aug 27, 2026 at 10:42 AM", MostRecent: true, BackupEligible: true, BackedUp: true}, {Display: "ready-key", UsageTracked: true, BackupEligible: true}, {Display: "device-key", UsageTracked: true}, {Display: "warning-key", UsageTracked: true, CloneWarning: true}, {Display: "legacy-key"}}, true, "/", "https://kinosail.test/account"}),
		"passkey-prompt":               localized(passkeyPromptView, nil),
		"mfa-required":                 localized(mfaRequiredView, nil),
		"oidc-mfa":                     localized(oidcMFAView, "fixture-challenge"),
		"viewing-import-preview":       localized(viewingImportPreviewView, preview),
		"viewing-import-result":        localized(viewingImportResultView, viewingImportSummary{Applied: 2, ListsApplied: 1, Unchanged: 3}),
		"mcp-approval":                 csrf(mcpApprovalView, mcpApprovalData{ClientName: "Living Room agent", ProfileName: "Owner", RequestID: "fixture-request", Write: true, Manage: true}),
		"supporter-populated-active":   localized(supporterView, supporterUIFixture(false)),
		"supporter-populated-archived": localized(supporterView, supporterUIFixture(true)),
		"state-contracts":              plain(template.Must(template.New("state-contracts").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>UI state contracts · Kinosail</title><link rel="stylesheet" href="/static/app.css?v=impeccable-1"></head><body class="settings-page"><main class="settings-shell"><header class="settings-intro"><span class="eyebrow">State contract</span><h1>System feedback</h1><p>Every state remains readable, calm, and explicit.</p></header><section class="wide"><h2>Empty</h2><div class="empty"><h2>Nothing here yet.</h2><p>Add Library Content to begin.</p></div></section><section aria-busy="true"><h2>Loading</h2><p role="status" class="status">Scanning the Library…</p><button disabled>Scanning</button></section><section><h2>Error</h2><div role="alert">The Server could not finish this request.</div></section><section><h2>Unavailable</h2><fieldset disabled aria-disabled="true"><label>Managed value <input value="Set by deployment"></label><button>Save</button></fieldset><p class="managed-setting">This setting is managed by the deployment.</p></section></main></body></html>`)), nil),
	}
	for name, render := range fixtures {
		var output bytes.Buffer
		if err := render(&output); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".html"), output.Bytes(), 0o600); err != nil { //nolint:gosec // Names are fixed above and the caller supplies the test-artifact directory.
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
