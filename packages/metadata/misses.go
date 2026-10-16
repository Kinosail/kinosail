package metadata

import (
	"math/rand/v2"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	firstMissBackoff   = time.Hour
	maximumMissBackoff = 7 * 24 * time.Hour
	maximumMisses      = 100_000
)

type Misses struct {
	mu      sync.Mutex
	entries map[string]miss
	now     func() time.Time
}

type miss struct {
	until time.Time
	delay time.Duration
}

func NewMisses() *Misses {
	return &Misses{entries: make(map[string]miss), now: time.Now}
}

func MissKey(item library.Item) string {
	if item.Show != "" {
		return item.Library + "\x00" + item.Show + "\x00" + item.ShowTitle
	}
	return item.ID + "\x00" + item.Title
}

func (misses *Misses) Blocked(item library.Item) bool {
	if misses == nil {
		return false
	}
	misses.mu.Lock()
	defer misses.mu.Unlock()
	entry, found := misses.entries[MissKey(item)]
	return found && misses.now().Before(entry.until)
}

func (misses *Misses) Fail(item library.Item) {
	if misses == nil {
		return
	}
	misses.mu.Lock()
	defer misses.mu.Unlock()
	key := MissKey(item)
	delay := firstMissBackoff
	if previous, found := misses.entries[key]; found {
		delay = min(maximumMissBackoff, previous.delay*2)
	} else if len(misses.entries) >= maximumMisses {
		clear(misses.entries)
	}
	jitter := time.Duration(rand.Int64N(int64(delay / 10))) //nolint:gosec // Retry jitter only spreads provider load.
	misses.entries[key] = miss{until: misses.now().Add(delay + jitter), delay: delay}
}

func (misses *Misses) Clear(item library.Item) {
	if misses == nil {
		return
	}
	misses.mu.Lock()
	delete(misses.entries, MissKey(item))
	misses.mu.Unlock()
}
