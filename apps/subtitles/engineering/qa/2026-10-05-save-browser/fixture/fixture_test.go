package main

import (
	"flag"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *fixtureServe {
		os.Exit(runFixture())
	}
	os.Exit(m.Run())
}

func TestSaveHeadersLegacyActualCompletion(t *testing.T)  { checkRealFault(t, "headers", false) }
func TestSaveBodyLegacyActualCompletion(t *testing.T)     { checkRealFault(t, "body", false) }
func TestSaveHeadersReceiptActualCompletion(t *testing.T) { checkRealFault(t, "headers", true) }
func TestSaveBodyReceiptActualCompletion(t *testing.T)    { checkRealFault(t, "body", true) }

func checkRealFault(t *testing.T, mode string, prepared bool) {
	t.Helper()
	target, err := newOwnedTarget()
	if err != nil {
		t.Fatal("real Server fixture construction failed")
	}
	t.Cleanup(func() {
		if !target.stop() {
			t.Error("owned fixture descriptor did not close")
		}
	})
	f, client, path, input, headers := bootstrapControl(t, target, mode, prepared)
	defer f.controlStop(t)
	exchange := startOwnedSave(t, target, client, path, input, headers)
	defer exchange.stop(t, f)
	awaitEligible(t, f, exchange)
	assertEligible(t, f.snapshot())
	var result responseReceipt
	if mode == "headers" {
		result = proveHeldHeaders(t, f, exchange)
	} else {
		result = proveHeldBody(t, f, exchange)
	}
	expected := http.StatusOK
	if prepared {
		expected = http.StatusAccepted
	}
	if result.status != expected || !equalApplicationHeaders(result.header, f.actualHeaders()) {
		t.Fatal("released actual response status or headers changed")
	}
	assertReleased(t, f)
}

func awaitEligible(t *testing.T, f *fixture, exchange *ownedSave) {
	t.Helper()
	select {
	case <-f.eligible:
	case <-exchange.failed:
		t.Fatal("Save transport prerequisite failed")
	case <-time.After(10 * time.Second):
		t.Fatal("actual Save/History/receipt witness missing")
	}
}

func assertEligible(t *testing.T, snapshot safeSnapshot) {
	t.Helper()
	if !snapshot.ActualSaved || !snapshot.HistoryOnce || !snapshot.RecoveryMatches {
		t.Fatal("actual persisted Save/History witness is not eligible")
	}
	if !snapshot.InspectionMatches || !snapshot.HoldEligible || snapshot.SaveAttempts != 1 {
		t.Fatal("actual persisted Save/History witness is not eligible")
	}
}

func assertReleased(t *testing.T, f *fixture) {
	t.Helper()
	settlement := time.Now().Add(2 * time.Second)
	snapshot := f.snapshot()
	for snapshot.ActiveHolds != 0 && time.Now().Before(settlement) {
		time.Sleep(10 * time.Millisecond)
		snapshot = f.snapshot()
	}
	if snapshot.SaveAttempts != 1 || !snapshot.ActualSaved || !snapshot.HistoryOnce {
		t.Fatal("release replayed or changed the actual Save")
	}
	if snapshot.ActiveHolds != 0 || !snapshot.ResponseBodyWritten || snapshot.ClientCancelled || snapshot.BoundaryFailed {
		t.Fatal("actual released response did not settle as a complete body write")
	}
}

const pureOwnedAuthority = "127.0.0.1:43107"

func pureAdmissionFixture() *fixture {
	target := &ownedTarget{
		authority: pureOwnedAuthority, origin: "https://" + pureOwnedAuthority,
		routes: make(map[string]url.URL),
	}
	target.seedFixedRoutes()
	return &fixture{target: target}
}

func pureAdmissionRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal("pure admission request construction failed")
	}
	return request
}

func assertPureOwnerAdmission(t *testing.T, f *fixture) {
	t.Helper()
	for _, path := range []string{"/setup", "/account"} {
		request := pureAdmissionRequest(t, "https://"+pureOwnedAuthority+path)
		if !f.admittedPrivateRequest(request) {
			t.Fatal("fixed Owner route unexpectedly rejected")
		}
	}
}

func assertPureAdmissionState(t *testing.T, f *fixture, before safeSnapshot) {
	t.Helper()
	if f.snapshot() != before {
		t.Error("pure admission changed fixture state")
	}
}

func TestOwnedRouteRejectsForeignTargets(t *testing.T) {
	f := pureAdmissionFixture()
	before := f.snapshot()
	defer assertPureAdmissionState(t, f, before)
	assertPureOwnerAdmission(t, f)
	cases := []struct{ name, target string }{
		{"external-https", "https://example.invalid/account"},
		{"http", "http://127.0.0.1:43107/account"},
		{"foreign-loopback-port", "https://127.0.0.1:43108/account"},
		{"protocol-relative", "//127.0.0.1:43107/account"},
		{"traversal-alias", "https://127.0.0.1:43107/../account"},
		{"query-alias", "https://127.0.0.1:43107/account?ignored=true"},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			request := pureAdmissionRequest(t, sample.target)
			if f.admittedPrivateRequest(request) {
				t.Fatal("foreign target was admitted")
			}
		})
	}
}

func TestOwnedRouteRejectsUnregisteredIDs(t *testing.T) {
	f := pureAdmissionFixture()
	before := f.snapshot()
	defer assertPureAdmissionState(t, f, before)
	assertPureOwnerAdmission(t, f)
	cases := []struct{ name, path string }{
		{"inspection", "/api/v1/subtitle-library/0123456789abcdef/inspect?language=en"},
		{"export", "/api/v1/subtitle-library/0123456789abcdef/export?language=en&format=srt"},
		{"preview", "/api/v1/subtitle-library/0123456789abcdef/preview"},
		{"apply", "/api/v1/subtitle-library/0123456789abcdef/apply"},
		{"receipt", "/api/v1/subtitle-operations/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			request := pureAdmissionRequest(t, "https://"+pureOwnedAuthority+sample.path)
			if f.admittedPrivateRequest(request) {
				t.Fatal("unregistered route was admitted")
			}
		})
	}
}
