package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupporterDisplayPersistenceAndFailure(t *testing.T) {
	t.Parallel()
	var saved installationSettings
	writes := 0
	store := &settingsStore{file: "settings.json", persist: func(_ string, value any) error { writes++; saved = value.(installationSettings); return nil }}
	for _, invalid := range []string{"", "HIDDEN", "hidden ", strings.Repeat("a", 129)} {
		if !errors.Is(store.setSupporterDisplay(invalid), errSupporterDisplay) {
			t.Fatal("invalid preference accepted")
		}
	}
	if writes != 0 {
		t.Fatal("invalid preference was persisted")
	}
	if err := store.setSupporterDisplay("hidden"); err != nil {
		t.Fatal(err)
	}
	restarted := &settingsStore{value: saved}
	if restarted.supporterDisplay() != "hidden" {
		t.Fatal("saved preference was lost")
	}
	store.persist = func(string, any) error { return errors.New("private path failed") }
	if err := store.setSupporterDisplay("automatic"); err == nil || store.supporterDisplay() != "hidden" {
		t.Fatal("failed save changed preference")
	}
	restarted.value.SupporterDisplay = "untrusted-import"
	if restarted.supporterDisplay() != "automatic" {
		t.Fatal("invalid imported preference was used")
	}
}

func TestSupporterFeaturedHonorSelection(t *testing.T) {
	t.Parallel()
	page := supporterBadgeCaseFixture()
	if page.Featured() != page.Living {
		t.Fatal("active recurring badge must lead")
	}
	page.Display = patronOrderFamily
	if page.Featured() != page.Patron {
		t.Fatal("chosen permanent badge must lead")
	}
	page.Display = "hidden"
	if page.Featured() != page.Living {
		t.Fatal("hiding navigation removed owned honors")
	}
	page.Living.Active = false
	page.Living.Expired = true
	if page.Featured() != page.Patron {
		t.Fatal("automatic chose an expired badge over permanent support")
	}
	page.Display = livingStandardFamily
	if page.Featured() != page.Living {
		t.Fatal("explicit archive selection was ignored")
	}
	page.Patron = nil
	page.Display = "automatic"
	if page.Featured() != page.Living {
		t.Fatal("archive disappeared")
	}
	page.Living = nil
	if page.Featured() != nil {
		t.Fatal("unearned badge appeared")
	}
}

func TestSailcraftArtworkIsDistinctSafeVectorAtEveryRank(t *testing.T) {
	t.Parallel()
	seen := map[[32]byte]string{}
	for _, family := range []string{patronOrderFamily, livingStandardFamily} {
		for rank := 1; rank <= 10; rank++ {
			for _, size := range []string{"", "-small"} {
				assertSafeDistinctBadge(t, fmt.Sprintf("static/supporter/badges/%s-%d%s.svg", family, rank, size), seen)
			}
			certificate, err := renderSupporterCertificate(supporterCertificateView{Name: "<Private & Grateful>", FamilySlug: family, Rank: rank})
			if err != nil || !strings.Contains(string(certificate), "Sailcraft") || strings.Count(string(certificate), `id="title"`) != 1 || strings.Contains(string(certificate), "<Private") {
				t.Fatalf("certificate not safe and self-contained for %s/%d", family, rank)
			}
		}
	}
	for _, input := range []struct {
		family string
		rank   int
	}{{"unknown", 1}, {"../../bad", 2}, {patronOrderFamily, 0}, {livingStandardFamily, 11}} {
		if supporterCertificateArt(input.family, input.rank) != "" {
			t.Fatal("invalid art selector accepted")
		}
	}
}

func supporterScenarioFixture(state string) supporterPageData {
	page := supporterBadgeCaseFixture()
	if state == "empty" || state == "patron" {
		page.Status.LivingStandard = nil
		page.Living = nil
		page.LivingLevels = supporterLevels(livingStandardFamily, nil, 0)
	}
	if state == "empty" || state == "living" || state == "archived" {
		page.Status.PatronOrder = nil
		page.Patron = nil
		page.PatronLevels = supporterLevels(patronOrderFamily, nil, 0)
	}
	if state == "archived" {
		page.Status.LivingStandard.Active = false
		page.Status.LivingStandard.Expired = true
		page.Living = supporterOwnedBadge(page.Status.LivingStandard)
		page.LivingLevels = supporterLevels(livingStandardFamily, page.Status.LivingStandard, 8)
	}
	if state == "long-name" {
		page.Living.RecognitionName = strings.Repeat("W", 80)
	}
	page.Masterwork = nil
	page.HasBadges = page.Patron != nil || page.Living != nil
	return page
}

func supporterOwnedCertificateFixture(family string) func(*bytes.Buffer) error {
	return func(output *bytes.Buffer) error {
		page := supporterBadgeCaseFixture()
		badge, name, titles := page.Status.PatronOrder, "Patron Order", patronBadgeTitles
		if family == livingStandardFamily {
			badge, name, titles = page.Status.LivingStandard, "Living Standard", livingBadgeTitles
		}
		data, err := renderSupporterCertificate(certificateView(badge, family, name, titles))
		if err == nil {
			_, err = output.Write(data)
		}
		return err
	}
}

func TestSupporterDisplaySaveFailureKeepsChoiceAndRendersRetry(t *testing.T) {
	store := &settingsStore{file: "settings.json", value: installationSettings{SupporterDisplay: "hidden"}, persist: func(string, any) error { return errors.New("private filesystem detail") }}
	program := newSupporterProgram(store, SupporterConfig{})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/display", strings.NewReader("display=automatic"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	program.saveSupporterDisplay(response, request)
	body := response.Body.String()
	if response.Code != http.StatusInternalServerError || response.Header().Get("Content-Type") != "text/html; charset=utf-8" || !strings.Contains(body, `role="alert"`) || !strings.Contains(body, "Please try again.") || strings.Contains(body, "private filesystem detail") {
		t.Fatalf("save failure did not provide a safe page: status=%d", response.Code)
	}
	if store.supporterDisplay() != "hidden" {
		t.Fatal("save failure changed recognition")
	}
}

func assertSafeDistinctBadge(t *testing.T, name string, seen map[[32]byte]string) {
	t.Helper()
	data, err := supporterBadges.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 32768 {
		t.Fatalf("oversized badge %s", name)
	}
	sum := sha256.Sum256(data)
	if previous, ok := seen[sum]; ok {
		t.Fatalf("%s reuses %s", name, previous)
	}
	seen[sum] = name
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		assertSafeBadgeElement(t, token, name)
	}
}

func assertSafeBadgeElement(t *testing.T, token xml.Token, name string) {
	t.Helper()
	start, ok := token.(xml.StartElement)
	if !ok {
		return
	}
	switch start.Name.Local {
	case "script", "image", "foreignObject", "animate", "set":
		t.Fatalf("unsafe or non-vector element %s in %s", start.Name.Local, name)
	}
	for _, attribute := range start.Attr {
		if strings.HasPrefix(attribute.Name.Local, "on") || attribute.Name.Local == "href" || strings.Contains(attribute.Value, "http") && attribute.Name.Local != "xmlns" {
			t.Fatalf("external or active attribute in %s", name)
		}
	}
}

func TestCurrentCertificatePrefersActiveEdition(t *testing.T) {
	status := supporterStatus{Monthly: &supporterBadgeStatus{Expired: true}, Yearly: &supporterBadgeStatus{Active: true}}
	if currentSupporterCertificate(status) != "yearly" {
		t.Fatal("expired monthly displaced active yearly")
	}
	status.Monthly.Active = true
	if currentSupporterCertificate(status) != "monthly" {
		t.Fatal("monthly should lead when active")
	}
	status.Monthly.Active = false
	status.Yearly.Active = false
	if currentSupporterCertificate(status) != "monthly" {
		t.Fatal("past support certificate disappeared")
	}
}
