package supporter

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestStatusChecksUnmigratedLegacyAgainstBothFamilies(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	status := service.Status(State{ActivationID: "invalid"})
	if status.PatronOrder != nil || status.LivingStandard != nil || status.Tier != "free" {
		t.Fatalf("invalid legacy status = %#v", status)
	}
}

func TestBadgeCaseClampsMalformedPersistedLevels(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	badgeCase := service.badgeCase(State{LivingLevel: -1, PatronLevel: MaximumLevel + 1}, nil, nil)
	if badgeCase.LivingLevel != 0 || badgeCase.PatronLevel != 0 || badgeCase.Unlocked != 0 {
		t.Fatalf("clamped badge case = %#v", badgeCase)
	}
}

func TestServiceMonthsHandlesInvalidExpiryAndCalendarBoundary(t *testing.T) {
	if months, marks := serviceMonths(Certificate{SupportedSince: "invalid"}, testNow()); months != 0 || marks != nil {
		t.Fatalf("invalid service months = %d, %#v", months, marks)
	}
	certificate := Certificate{SupportedSince: "2025-01-01T00:00:00Z", ExpiresAt: "2025-07-01T00:00:00Z"}
	if months, marks := serviceMonths(certificate, testNow()); months != 6 || len(marks) != 2 {
		t.Fatalf("expired service months = %d, %#v", months, marks)
	}
	certificate = Certificate{SupportedSince: "2026-01-31T00:00:00Z"}
	end := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	if months, _ := serviceMonths(certificate, end); months != 0 {
		t.Fatalf("partial calendar month = %d", months)
	}
}

func TestCertificateForLivingFamilyAndUnknownFamily(t *testing.T) {
	now := testNow()
	service := testServiceAt(t, App{}, now)
	if _, _, ok := service.CertificateForFamily(State{}, "unknown"); ok {
		t.Fatal("unknown certificate family was accepted")
	}
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	value := currentCertificateValue(now, installation)
	value["tier"], value["sustaining"], value["expiresAt"] = "admiral", true, now.Add(30*24*time.Hour).Format(time.RFC3339Nano)
	grant := legacySignedGrant(t, value)
	certificate, badge, ok := service.CertificateForFamily(State{InstallationKey: installation, LivingStandard: &grant}, FamilyLiving)
	if !ok || badge == nil || !certificate.Sustaining || badge.Rank != Rank("admiral") {
		t.Fatalf("living certificate = %#v, %#v, %t", certificate, badge, ok)
	}
}
