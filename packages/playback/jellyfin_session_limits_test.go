package playback

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestJellyfinSessionsAreBoundedUnderConcurrentCreation(t *testing.T) {
	now := time.Now()
	session := testJellyfinSession{item: "movie", profile: "viewer", expires: now.Add(time.Hour)}
	var store sync.Map
	for i := range maximumJellyfinSessions - 1 {
		store.Store(fmt.Sprint(i), session)
	}
	var calls sync.WaitGroup
	for i := range 10 {
		calls.Go(func() {
			_, _ = StoreJellyfinPlaySession(&store, now, func() (string, error) { return fmt.Sprintf("new-%d", i), nil }, session)
		})
	}
	calls.Wait()
	count := 0
	store.Range(func(_, _ any) bool { count++; return true })
	if count != maximumJellyfinSessions {
		t.Fatalf("unbounded session count: %d", count)
	}
	if _, found := LoadJellyfinPlaySession(&store, "0", now.Add(2*time.Hour)); found {
		t.Fatal("expired session returned")
	}
	if _, found := store.Load("0"); found {
		t.Fatal("expired session retained after access")
	}
	if _, err := StoreJellyfinPlaySession(&store, now, func() (string, error) { return "replacement", nil }, session); err != nil {
		t.Fatal(err)
	}
}
