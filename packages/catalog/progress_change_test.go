package catalog

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProgressRevisionRejectsChangesWithoutPersistenceOrAudit(t *testing.T) {
	now := time.Unix(100, 0)
	for _, test := range []struct {
		name     string
		seconds  float64
		session  string
		revision uint64
	}{{"stale", 1, "phone", 1}, {"duplicate", 1, "phone", 2}, {"negative", -1, "phone", 3}, {"nan", math.NaN(), "phone", 3}, {"infinite", math.Inf(1), "phone", 3}, {"oversized", 1e9 + 1, "phone", 3}, {"session", 5, strings.Repeat("x", 129), 3}} {
		t.Run(test.name, func(t *testing.T) {
			original := PlaybackState{Seconds: 120, Session: "phone", Revision: 2, Updated: now}
			values := map[string]PlaybackState{"viewer:item": original}
			storage := ProgressStorage{Mutex: new(sync.Mutex), PersistMutex: new(sync.Mutex), Values: &values, File: "progress.json", Persist: func(string, any) error { t.Fatal("rejected event persisted"); return nil }}
			watched := true
			previous, current, accepted, err := storage.Update("viewer:item", ProgressRevision(test.seconds, &watched, test.session, test.revision, now.Add(time.Hour)))
			started, completed := ProgressAudit(previous, current, test.seconds, &watched, accepted, err)
			if accepted || started || completed || !reflect.DeepEqual(values["viewer:item"], original) {
				t.Fatalf("rejected event changed state or audit: %v %v %v", accepted, started, completed)
			}
		})
	}
}

func TestProgressRevisionResetsFlagsAndPreservesSession(t *testing.T) {
	now := time.Unix(200, 0)
	state, accepted, err := ProgressRevision(10, nil, "new", 3, now)(PlaybackState{Watched: true, Dismissed: true})
	if err != nil || !accepted {
		t.Fatalf("revision accepted=%v error=%v", accepted, err)
	}
	assertProgressRevisionReset(t, state, now)
	state, _, _ = ProgressRevision(0, nil, "", 0, now)(state)
	if state.Session != "new" || state.Revision != 3 {
		t.Fatal("unversioned event cleared session")
	}
}

func assertProgressRevisionReset(t *testing.T, state PlaybackState, now time.Time) {
	t.Helper()
	if state.Seconds != 10 || state.Watched || state.Dismissed || state.Session != "new" || state.Revision != 3 || !state.Updated.Equal(now) {
		t.Fatalf("state = %#v", state)
	}
}

func TestProgressAuditTransitions(t *testing.T) {
	now := time.Unix(200, 0)
	watch := true
	for _, test := range []struct {
		previous           PlaybackState
		seconds            float64
		watched            *bool
		err                error
		started, completed bool
	}{
		{PlaybackState{}, 1, nil, nil, true, false},
		{PlaybackState{Updated: now, Seconds: 100}, 69, nil, nil, true, false},
		{PlaybackState{Updated: now, Seconds: 100}, 70, nil, nil, false, false},
		{PlaybackState{Updated: now.Add(-30 * time.Minute)}, 1, nil, nil, true, false},
		{PlaybackState{Updated: now}, 0, &watch, nil, false, true},
		{PlaybackState{Updated: now, Watched: true}, 0, &watch, nil, false, false},
		{PlaybackState{}, 1, &watch, errors.New("persist"), false, false},
	} {
		started, completed := ProgressAudit(test.previous, PlaybackState{Updated: now}, test.seconds, test.watched, true, test.err)
		if started != test.started || completed != test.completed {
			t.Fatalf("audit = %v %v for %#v", started, completed, test)
		}
	}
}
