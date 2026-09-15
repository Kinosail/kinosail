// Package viewing owns source-neutral viewing import matching and change planning.
package viewing

import (
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

// Input selects one remote source and one destination Viewer Profile.
type Input struct {
	Source     string `json:"source"`
	URL        string `json:"url"`
	Token      string `json:"token"`
	SourceUser string `json:"sourceUser,omitempty"`
	ProfileID  string `json:"profileId"`
	Overwrite  bool   `json:"overwriteExisting,omitempty"`
}

// Activity is one normalized item returned by a supported source.
type Activity struct {
	SourceID, Kind, Title, Year, Show, Path string
	Season, Episode                         int
	ProviderIDs                             map[string]string
	Playlists                               map[string]int
	Seconds, Duration                       float64
	Watched, Favorite                       bool
	Updated                                 time.Time
}

// Item describes one preview decision without source credentials.
type Item struct {
	SourceTitle string   `json:"sourceTitle"`
	TargetID    string   `json:"targetId,omitempty"`
	TargetTitle string   `json:"targetTitle,omitempty"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason"`
	Seconds     float64  `json:"seconds,omitempty"`
	Watched     bool     `json:"watched"`
	Favorite    bool     `json:"favorite,omitempty"`
	Playlists   []string `json:"playlists,omitempty"`
}

// Summary counts every source, match, conflict, and applied outcome.
type Summary struct {
	SourceItems   int `json:"sourceItems"`
	Activity      int `json:"activity"`
	Matched       int `json:"matched"`
	Importable    int `json:"importable"`
	Unchanged     int `json:"unchanged"`
	Ambiguous     int `json:"ambiguous"`
	Unmatched     int `json:"unmatched"`
	Conflicts     int `json:"conflicts"`
	Favorites     int `json:"favorites,omitempty"`
	PlaylistItems int `json:"playlistItems,omitempty"`
	Applied       int `json:"applied,omitempty"`
	ListsApplied  int `json:"listsApplied,omitempty"`
}

// ProgressChange is an optimistic per-item playback update.
type ProgressChange struct {
	TargetID string
	State    catalog.PlaybackState
	Expected catalog.PlaybackState
}

// ListChange adds a matched item to My List or source playlists.
type ListChange struct {
	TargetID  string
	Favorite  bool
	Playlists map[string]int
}

// Plan is the public preview and its private optimistic change set.
type Plan struct {
	Summary     Summary
	Items       []Item
	Truncated   bool
	Changes     []ProgressChange
	ListChanges []ListChange
}

// Preview is one expiring source snapshot and its optimistic change plan.
type Preview struct {
	ID          string           `json:"id"`
	Source      string           `json:"source"`
	ProfileID   string           `json:"profileId"`
	ProfileName string           `json:"profileName"`
	ExpiresAt   time.Time        `json:"expiresAt"`
	Summary     Summary          `json:"summary"`
	Items       []Item           `json:"items"`
	Truncated   bool             `json:"truncated,omitempty"`
	Input       Input            `json:"-"`
	Activities  []Activity       `json:"-"`
	Changes     []ProgressChange `json:"-"`
	ListChanges []ListChange     `json:"-"`
	Timer       *time.Timer      `json:"-"`
}

// PreviewStore coordinates bounded work and expiring previews under an app-owned lock.
type PreviewStore struct {
	Mutex          *sync.Mutex
	Values         *map[string]Preview
	Slots          *chan struct{}
	Limit, Workers int
}

// Observation records the last source signature and its stable destination match.
type Observation struct {
	TargetID  string `json:"targetId"`
	Signature string `json:"signature"`
}

// SyncPending is a durable fetched batch awaiting finalization.
type SyncPending struct {
	Summary     Summary          `json:"summary"`
	Changes     []ProgressChange `json:"changes,omitempty"`
	ListChanges []ListChange     `json:"listChanges,omitempty"`
}

// Sync is one durable recurring source reconciliation.
type Sync struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	URL        string                 `json:"url"`
	Token      string                 `json:"token"`
	SourceUser string                 `json:"sourceUser,omitempty"`
	ProfileID  string                 `json:"profileId"`
	Interval   string                 `json:"interval"`
	LastRun    time.Time              `json:"lastRun,omitempty"`
	NextRun    time.Time              `json:"nextRun,omitempty"`
	LastError  string                 `json:"lastError,omitempty"`
	LastResult Summary                `json:"lastResult"`
	Seen       map[string]Observation `json:"seen,omitempty"`
	Pending    *SyncPending           `json:"pending,omitempty"`
	Running    bool                   `json:"-"`
	Queued     bool                   `json:"-"`
}

// SyncView is the credential-free presentation of one recurring import.
type SyncView struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"`
	URL         string  `json:"url"`
	SourceUser  string  `json:"sourceUser,omitempty"`
	ProfileID   string  `json:"profileId"`
	ProfileName string  `json:"profileName"`
	Interval    string  `json:"interval"`
	LastRun     string  `json:"lastRun,omitempty"`
	NextRun     string  `json:"nextRun,omitempty"`
	LastError   string  `json:"lastError,omitempty"`
	LastResult  Summary `json:"lastResult"`
	Running     bool    `json:"running"`
}
