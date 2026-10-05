package main

import (
	"flag"
	"net/http"
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
	f, client, path, input, headers := bootstrapControl(t, mode, prepared)
	defer f.controlStop(t)
	exchange := startOwnedSave(t, f, client, path, input, headers)
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
