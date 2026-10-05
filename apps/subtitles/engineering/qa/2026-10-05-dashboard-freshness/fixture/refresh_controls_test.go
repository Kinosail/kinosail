package main

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// Failure-first controls. newR16PublicFixture is intentionally not implemented
// by this source package; root must integrate and review the actual Server,
// Owner/MFA/CSRF, finite target, SSE, retained filesystem and settlement helpers.
// These declarations have not been compiled or executed.

// Required root-owned implementation seam. Request returns bounded, consumed,
// closed response values; Files uses only the retained owned os.Root. OpenEvents
// reads real Server SSE frames, not private Publish or synthetic events.
type r16PublicFixture interface {
	ItemID() string
	Request(context.Context, string, string, []byte, r16Authority) r16Response
	OpenEvents(context.Context, string, time.Duration) r16EventStream
	Files(*testing.T) r16Files
	ProviderCalls() int
	StopAndJoin(context.Context) bool
}

type r16EventStream interface {
	Status() int
	Headers() http.Header
	Next(context.Context) (r16Frame, error)
	WaitClosed(context.Context) bool
	StopAndJoin(context.Context) bool
}

type r16Response struct {
	Status int
	Body []byte
	Complete bool
}

type r16Files struct {
	CurrentExists bool
	Current []byte
	Recovery []byte
	RecoveryExists bool
}

type r16Frame struct {
	ID string
	Type string
	Data []byte
}

type r16Event struct {
	ID uint64 `json:"id"`
	Type string `json:"type"`
	Resource string `json:"resource"`
	At time.Time `json:"at"`
}

type r16Counts struct {
	Total int `json:"total"`
	Ready int `json:"ready"`
	Wanted int `json:"wanted"`
	Pending int `json:"pending"`
	Unavailable int `json:"unavailable"`
}

type r16Row struct {
	ID string `json:"id"`
	State string `json:"state"`
	Source string `json:"source"`
	Installed string `json:"installed"`
	Ready bool `json:"ready"`
	Frozen bool `json:"frozen"`
	Restorable bool `json:"restorable"`
}

type r16History struct {
	ID string `json:"id"`
	Action string `json:"action"`
	Language string `json:"language"`
	Source string `json:"source"`
	Reason string `json:"reason"`
	Changed string `json:"changed"`
	Available bool `json:"available"`
}

type r16Provider struct {
	Name string `json:"name"`
	State string `json:"state"`
	Configured bool `json:"configured"`
	Tested bool `json:"tested"`
	LastSuccess string `json:"lastSuccess"`
	Remaining *int `json:"remaining"`
}

type r16ProviderResult struct {
	Attempted int `json:"attempted"`
	Connected int `json:"connected"`
}

func r16AssertEmptyCounts(t *testing.T, counts r16Counts) {
	t.Helper()
	if counts != (r16Counts{}) {
		t.Fatal("provider-only fixture did not have an actually empty catalogue")
	}
}

func r16AssertEmptyFiles(t *testing.T, files r16Files) {
	t.Helper()
	if files.CurrentExists || files.RecoveryExists || len(files.Current) != 0 || len(files.Recovery) != 0 {
		t.Fatal("provider-only fixture unexpectedly contained sidecar or recovery files")
	}
}

func r16Dialogue(text string) string {
	switch text {
	case r16ReplacementText:
		return "R16 corrected line."
	case r16ReplayText:
		return "R16 replay witness."
	default:
		return ""
	}
}

func TestR16ReadyToReadyLedgerEventPublicControl(t *testing.T) {
	f := newR16PublicFixture(t, false)
	defer r16SettleFixture(t, f)
	before := r16ReadCounts(t, f)
	r16AssertReadyCounts(t, before)
	row := r16ReadRow(t, f)
	r16AssertInitialRow(t, row, f.ItemID())
	r16AssertHistoryEmpty(t, f)
	r16AssertFiles(t, f.Files(t), r16InitialText, "", false)
	stream := r16OpenStream(t, f, "")
	defer r16SettleStream(t, stream)
	r16Save(t, f, r16ReplacementText)
	r16AssertCountsEqual(t, before, r16ReadCounts(t, f))
	r16AssertSavedRow(t, r16ReadRow(t, f), f.ItemID())
	r16AssertManualHistory(t, f, 1)
	r16AssertFiles(t, f.Files(t), r16ReplacementText, r16InitialText, true)
	event := r16NextSubtitleEvent(t, stream)
	r16AssertSubtitleEvent(t, event)
}

func TestR16EventReplayAndRevocationPublicControl(t *testing.T) {
	f := newR16PublicFixture(t, false)
	defer r16SettleFixture(t, f)
	r16AssertReadyCounts(t, r16ReadCounts(t, f))
	stream := r16OpenStream(t, f, "")
	defer r16SettleStream(t, stream)
	r16Save(t, f, r16ReplacementText)
	first := r16NextSubtitleEvent(t, stream)
	r16AssertSubtitleEvent(t, first)
	r16SettleStream(t, stream)
	r16Save(t, f, r16ReplayText)
	r16AssertManualHistory(t, f, 2)
	replay := r16OpenStream(t, f, strconv.FormatUint(first.ID, 10))
	defer r16SettleStream(t, replay)
	second := r16NextSubtitleEvent(t, replay)
	r16AssertSubtitleEvent(t, second)
	if second.ID <= first.ID {
		t.Fatal("retained subtitle event did not advance the actual cursor")
	}
	response := r16Request(t, f, http.MethodDelete, "/api/v1/session", nil, r16Owner)
	if response.Status != http.StatusNoContent || len(response.Body) != 0 {
		t.Fatal("actual disposable Owner session was not revoked")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if !replay.WaitClosed(ctx) {
		t.Fatal("revoked session event stream did not terminate and join")
	}
	rejected := r16Request(t, f, http.MethodGet, "/api/v1/events", nil, r16Owner)
	if rejected.Status != http.StatusUnauthorized {
		t.Fatal("revoked session retained event access")
	}
}

func TestR16ProviderHealthOnlyPublicControl(t *testing.T) {
	f := newR16PublicFixture(t, true)
	defer r16SettleFixture(t, f)
	before := r16ReadCounts(t, f)
	r16AssertEmptyCounts(t, before)
	files := f.Files(t)
	r16AssertEmptyFiles(t, files)
	r16AssertHistoryEmpty(t, f)
	health := r16ReadProvider(t, f)
	if !health.Configured || health.Tested || health.LastSuccess != "" {
		t.Fatal("owned provider did not begin configured and untested")
	}
	calls := f.ProviderCalls()
	response := r16Request(t, f, http.MethodPost, "/api/v1/subtitle-providers/test", []byte("{}"), r16Owner)
	var result r16ProviderResult
	r16DecodeResponse(t, response, &result)
	if result.Attempted != 1 || result.Connected != 1 {
		t.Fatal("actual owned provider credential test did not connect")
	}
	if f.ProviderCalls() != calls+1 {
		t.Fatal("provider health was not produced by one actual owned request")
	}
	r16AssertConnectedProvider(t, r16ReadProvider(t, f))
	r16AssertCountsEqual(t, before, r16ReadCounts(t, f))
	r16AssertFilesEqual(t, files, f.Files(t))
	r16AssertHistoryEmpty(t, f)
	// This control does not turn the current missing provider publisher into
	// a permanent no-event requirement or claim an automatic upgrade.
}

func TestR16UnauthorizedFreshnessRequestsNoEffects(t *testing.T) {
	f := newR16PublicFixture(t, true)
	defer r16SettleFixture(t, f)
	before := r16ReadCounts(t, f)
	r16AssertEmptyCounts(t, before)
	files := f.Files(t)
	r16AssertEmptyFiles(t, files)
	calls := f.ProviderCalls()
	r16AssertHistoryEmpty(t, f)
	for _, route := range []string{
		"/api/v1/events",
		"/api/v1/subtitle-library?view=summary",
		"/api/v1/subtitle-library?view=history",
	} {
		response := r16Request(t, f, http.MethodGet, route, nil, r16Anonymous)
		if response.Status != http.StatusUnauthorized {
			t.Fatal("anonymous freshness read was not rejected")
		}
	}
	for _, authority := range []r16Authority{r16MissingCSRF, r16WrongCSRF} {
		response := r16Request(t, f, http.MethodPost, "/api/v1/subtitle-providers/test", []byte("{}"), authority)
		if response.Status != http.StatusForbidden {
			t.Fatal("invalid CSRF provider mutation was not rejected")
		}
	}
	r16AssertCountsEqual(t, before, r16ReadCounts(t, f))
	r16AssertFilesEqual(t, files, f.Files(t))
	r16AssertHistoryEmpty(t, f)
	if f.ProviderCalls() != calls {
		t.Fatal("rejected freshness requests reached the owned provider")
	}
}
