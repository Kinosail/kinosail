package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupporterChooserKeepsHonorsAndActivationRecovery(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"empty", "living", "patron", "archived", "long-name"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			page := supporterScenarioFixture(state)
			for _, message := range []string{"", "The supporter key could not be read."} {
				assertSupporterChooser(t, page, message)
			}
		})
	}
}

func assertSupporterChooser(t *testing.T, page supporterPageData, message string) {
	t.Helper()
	page.Error = message
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/supporter", nil)
	if err := supporterView.Execute(response, request, page); err != nil {
		t.Fatal(err)
	}
	body := response.Body.String()
	if strings.Count(body, `data-supporter-price="`) != 20 || strings.Count(body, `class="supporter-gallery"`) != 2 {
		t.Fatal("chooser must include all ten levels in both families")
	}
	if strings.Contains(body, `supporter-all-levels`) || strings.Contains(body, `Show all 10 levels`) {
		t.Fatal("all levels must be visible without an expansion control")
	}
	if !strings.Contains(body, `data-supporter-family="patron-order" hidden`) || !strings.Contains(body, "Continue to support site") || !strings.Contains(body, "<noscript>") {
		t.Fatal("default recurring family or support-site fallback missing")
	}
	if strings.Contains(body, `class="supporter-activation-disclosure" open`) != (message != "") {
		t.Fatal("activation form must reopen after an error and start collapsed otherwise")
	}
	assertSupporterCertificateActions(t, page, body)
}

func assertSupporterCertificateActions(t *testing.T, page supporterPageData, body string) {
	t.Helper()
	for _, family := range []string{"Living Standard", "Patron Order"} {
		expected := 0
		if family == "Living Standard" && page.Living != nil || family == "Patron Order" && page.Patron != nil {
			expected = 1
		}
		if strings.Count(body, "Download "+family+" certificate</a>") != expected {
			t.Fatalf("%s certificate action missing or duplicated", family)
		}
	}
}
