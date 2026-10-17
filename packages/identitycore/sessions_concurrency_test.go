package identitycore

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"
)

type recordingSessionStore struct {
	mu     sync.Mutex
	values map[string]Session
	last   []byte
	writes int
	hold   func()
}

func (store *recordingSessionStore) sessions() *Sessions {
	return NewSessions(SessionConfig{
		Mutex: &sync.RWMutex{}, Values: &store.values, File: "sessions.json",
		Persist: func(_ string, value any) error {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if store.hold != nil {
				store.hold()
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			store.last, store.writes = data, store.writes+1
			return nil
		},
		Profiles: func() []SessionProfile { return []SessionProfile{{ID: "viewer", Name: "Sam"}} },
	})
}

func TestSessionWritesDoNotBlockAuthenticatedReaders(t *testing.T) {
	persisting, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	store := &recordingSessionStore{values: map[string]Session{}}
	store.hold = func() {
		once.Do(func() { close(persisting) })
		<-release
	}
	sessions := store.sessions()
	created := make(chan error, 1)
	go func() {
		_, err := sessions.Create("viewer", "TV", true, false, "")
		created <- err
	}()
	<-persisting
	read := make(chan int, 1)
	go func() { read <- sessions.Active() }()
	select {
	case active := <-read:
		if active != 1 {
			t.Errorf("active sessions during persistence = %d", active)
		}
	case <-time.After(2 * time.Second):
		t.Error("authenticated reader waited for session persistence")
	}
	close(release)
	if err := <-created; err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentLoginsAndSignOutsPersistTheLatestSessions(t *testing.T) {
	store := &recordingSessionStore{values: map[string]Session{}}
	sessions := store.sessions()
	const devices = 48
	kept := make([]string, devices)
	var workers sync.WaitGroup
	for device := range devices {
		workers.Go(func() {
			token, err := sessions.Create("viewer", "Device", true, false, "")
			if err != nil {
				t.Error(err)
				return
			}
			if device%2 == 0 {
				if err := sessions.SignOut(token); err != nil {
					t.Error(err)
				}
				return
			}
			kept[device] = SessionKey(token)
		})
		workers.Go(func() { _ = sessions.Active() })
	}
	workers.Wait()
	var persisted map[string]Session
	if err := json.Unmarshal(store.last, &persisted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, store.values) || len(persisted) != devices/2 {
		t.Fatalf("last persisted %d sessions, memory has %d", len(persisted), len(store.values))
	}
	for device, key := range kept {
		if _, found := persisted[key]; device%2 == 1 && !found {
			t.Fatalf("session %d was lost", device)
		}
	}
	if store.writes != devices+devices/2 {
		t.Fatalf("writes = %d, want one per login and sign-out", store.writes)
	}
}
