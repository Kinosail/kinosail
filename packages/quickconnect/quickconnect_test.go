package quickconnect

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrokerCompletesOneAuthorizationLifecycle(t *testing.T) { //nolint:cyclop // One interface test covers the complete one-time authorization lifecycle.
	t.Parallel()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	broker := New(time.Minute)
	broker.now = func() time.Time { return now }
	secret, connection, err := broker.Create(Request{Device: "Bedroom TV", Client: "Kinosail", Version: "1"})
	if err != nil || !strings.HasPrefix(secret, "qc_") || len(connection.Code) != 8 || connection.Created != now {
		t.Fatalf("Create() = %q, %#v, %v", secret, connection, err)
	}
	if _, err = broker.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatalf("Consume() before approval error = %v", err)
	}
	viewer := Viewer{ID: "viewer", Revision: 7}
	if err = broker.Approve("  "+strings.ToLower(connection.Code)+"  ", viewer); err != nil {
		t.Fatal(err)
	}
	if err = broker.Approve(connection.Code, viewer); err != nil {
		t.Fatalf("repeated approval by the same Viewer = %v", err)
	}
	if err = broker.Approve(connection.Code, Viewer{ID: "other"}); !errors.Is(err, ErrAlreadyApproved) {
		t.Fatalf("approval by another Viewer error = %v", err)
	}
	status, found := broker.Status(secret)
	if !found || !status.Approved {
		t.Fatalf("Status() = %#v, %v", status, found)
	}
	grant, err := broker.Consume(secret)
	if err != nil || grant.ProfileID != viewer.ID || grant.Device != "Bedroom TV" || grant.ProfileRevision != viewer.Revision || grant.Remote {
		t.Fatalf("Consume() = %#v, %v", grant, err)
	}
	if _, found = broker.Status(secret); found {
		t.Fatal("consumed request remains available")
	}
	if _, err = broker.Consume(secret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reused secret error = %v", err)
	}
}

func TestRemoteApprovalRequiresOneSecureRemoteViewer(t *testing.T) {
	t.Parallel()
	invalid := []Viewer{
		{ID: "owner", Owner: true, Remote: true, Secured: true, StronglyVerified: true},
		{ID: "local", Secured: true, StronglyVerified: true},
		{ID: "password", Remote: true, StronglyVerified: true},
		{ID: "stale", Remote: true, Secured: true},
	}
	for _, viewer := range invalid {
		t.Run(viewer.ID, func(t *testing.T) {
			broker := New(time.Minute)
			secret, connection, err := broker.Create(Request{Device: "TV", Remote: true})
			if err != nil {
				t.Fatal(err)
			}
			if err = broker.Approve(connection.Code, viewer); !errors.Is(err, ErrRemoteViewer) {
				t.Fatalf("Approve() error = %v", err)
			}
			if _, err = broker.Consume(secret); !errors.Is(err, ErrPending) {
				t.Fatalf("rejected approval changed request: %v", err)
			}
		})
	}

	broker := New(time.Minute)
	secret, connection, err := broker.Create(Request{Device: "TV", Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	viewer := Viewer{ID: "viewer", Revision: 4, Remote: true, Secured: true, StronglyVerified: true}
	if err = broker.Approve(connection.Code, viewer); err != nil {
		t.Fatal(err)
	}
	grant, err := broker.Consume(secret)
	if err != nil || !grant.Remote || grant.ProfileRevision != viewer.Revision {
		t.Fatalf("Consume() = %#v, %v", grant, err)
	}
}

func TestBrokerExpiresBoundsAndRevokesRequests(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	broker := New(time.Minute)
	broker.now = func() time.Time { return now }
	localSecret, _, err := broker.Create(Request{Device: "Local"})
	if err != nil {
		t.Fatal(err)
	}
	remoteSecret, _, err := broker.Create(Request{Device: "Remote", Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	broker.RevokeRemote()
	if _, found := broker.Status(remoteSecret); found {
		t.Fatal("remote request survived revocation")
	}
	if _, found := broker.Status(localSecret); !found {
		t.Fatal("local request was revoked")
	}
	now = now.Add(time.Minute)
	if _, found := broker.Status(localSecret); found {
		t.Fatal("request survived its expiry instant")
	}

	for range maxPending {
		if _, _, err = broker.Create(Request{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = broker.Create(Request{}); !errors.Is(err, ErrCapacity) {
		t.Fatalf("request above capacity error = %v", err)
	}
	now = now.Add(time.Minute)
	if _, _, err = broker.Create(Request{}); err != nil {
		t.Fatalf("expired requests did not release capacity: %v", err)
	}
}

func TestNumericCodeUsesSixDigits(t *testing.T) {
	t.Parallel()
	_, connection, err := New(time.Minute).Create(Request{Numeric: true})
	if err != nil || len(connection.Code) != 6 {
		t.Fatalf("Create() = %#v, %v", connection, err)
	}
	for _, character := range connection.Code {
		if character < '0' || character > '9' {
			t.Fatalf("numeric code = %q", connection.Code)
		}
	}
}

func TestBrokerRejectsUnboundedOrAmbiguousInputBeforeCapacity(t *testing.T) { //nolint:cyclop,gocognit // Scores of 16 and 17 remain below the repository ceiling of 22 for the rejection matrix.
	t.Parallel()
	broker := New(time.Minute)
	codeCalls := 0
	broker.code = func(bool) (string, error) {
		codeCalls++
		return "BOUNDARY", nil
	}
	for _, request := range []Request{
		{Device: strings.Repeat("d", 81)},
		{DeviceID: strings.Repeat("i", 129)},
		{Client: "client\nname"},
		{Version: strings.Repeat("v", 41)},
	} {
		if _, _, err := broker.Create(request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("Create(%#v) error = %v", request, err)
		}
	}
	if codeCalls != 0 || len(broker.pending) != 0 || len(broker.byCode) != 0 {
		t.Fatalf("invalid requests changed broker: codes=%d pending=%d indexed=%d", codeCalls, len(broker.pending), len(broker.byCode))
	}
	maximum := Request{Device: strings.Repeat("d", 80), DeviceID: strings.Repeat("i", 128), Client: strings.Repeat("c", 80), Version: strings.Repeat("v", 40)}
	secret, connection, err := broker.Create(maximum)
	if err != nil || connection.Device != maximum.Device || codeCalls != 1 {
		t.Fatalf("maximum request = %#v, %v, code calls=%d", connection, err, codeCalls)
	}
	for _, invalidSecret := range []string{strings.Repeat("s", 129), "secret\n"} {
		if _, found := broker.Status(invalidSecret); found {
			t.Fatalf("invalid secret %q was found", invalidSecret[:min(len(invalidSecret), 20)])
		}
		if _, consumeErr := broker.Consume(invalidSecret); !errors.Is(consumeErr, ErrNotFound) {
			t.Fatalf("invalid secret consume error = %v", consumeErr)
		}
	}
	invalidApprovals := []struct {
		code   string
		viewer Viewer
	}{
		{strings.Repeat("c", 17), Viewer{ID: "viewer"}},
		{connection.Code, Viewer{}},
		{connection.Code, Viewer{ID: strings.Repeat("v", 129)}},
		{connection.Code, Viewer{ID: "viewer\n"}},
	}
	for _, approval := range invalidApprovals {
		if err := broker.Approve(approval.code, approval.viewer); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid approval error = %v", err)
		}
	}
	if status, found := broker.Status(secret); !found || status.Approved {
		t.Fatalf("invalid input changed valid request: %#v, found=%t", status, found)
	}
}

func TestApprovedRequestCanBeConsumedOnlyOnceConcurrently(t *testing.T) {
	t.Parallel()
	broker := New(time.Minute)
	secret, connection, err := broker.Create(Request{Device: "TV"})
	if err != nil {
		t.Fatal(err)
	}
	if err = broker.Approve(connection.Code, Viewer{ID: "viewer"}); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			if _, consumeErr := broker.Consume(secret); consumeErr == nil {
				successes.Add(1)
			} else if !errors.Is(consumeErr, ErrNotFound) {
				t.Errorf("Consume() error = %v", consumeErr)
			}
		})
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful consumes = %d", successes.Load())
	}
}
