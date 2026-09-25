package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

func TestDLNALabelExplainsUnconfiguredAndConfiguredStates(t *testing.T) {
	settings := newSettingsStore("", "", "", nil)
	servertest.AssertDLNALabelExplainsUnconfiguredAndConfiguredStates(t, func() string { return dlnaLabel(settings) }, func(url, token string) { settings.dlnaURL = url; settings.value.DLNAToken = token })
}

func TestHardwareSupportAcceptsAutomaticAndKnownBackends(t *testing.T) {
	capabilities := hardwareCapabilities{Selected: "software", Backends: []hardwareBackend{{ID: "vaapi", Supported: true}}}
	servertest.AssertHardwareSupportAcceptsAutomaticAndKnownBackends(t, capabilities.Supports)
}

func TestPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T) {
	servertest.AssertByteSizeBoundaries(t, byteSize)
}

func TestAvailableUpdateAddsOwnerOnlyLibraryIndicator(t *testing.T) {
	settings := newSettingsStore("", "", "", nil)
	checker, err := updatecontrol.NewChecker(updatecontrol.CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: settings.updateChecks, SaveAutomatic: settings.setUpdateChecks,
		Source: updatecontrol.ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			return "v1.1.0", "", false, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	checker.Check(t.Context())
	index := memoryLibraryIndex(nil, true)
	handler := showHome(index, newProgressStore(""), newListStore(""), settings, checker, false)
	owner := httptest.NewRecorder()
	handler(owner, ownerRequest("/"))
	if !strings.Contains(owner.Body.String(), "Settings · Update") {
		t.Fatalf("Owner page has no update indicator: %q", owner.Body.String())
	}
	viewerRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	viewerRequest = viewerRequest.WithContext(context.WithValue(viewerRequest.Context(), viewerContextKey{}, viewerProfile{ID: "viewer", Name: "Viewer"}))
	viewer := httptest.NewRecorder()
	handler(viewer, viewerRequest)
	if strings.Contains(viewer.Body.String(), "Settings · Update") {
		t.Fatal("Viewer received the Owner update indicator")
	}
}
