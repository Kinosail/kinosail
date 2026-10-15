package viewing

import (
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

type Input struct {
	Source     string `json:"source"`
	URL        string `json:"url"`
	Token      string `json:"token"`
	SourceUser string `json:"sourceUser,omitempty"`
	ProfileID  string `json:"profileId"`
	Overwrite  bool   `json:"overwriteExisting,omitempty"`
}

type Activity struct {
	SourceID, Kind, Title, Year, Show, Path string
	Season, Episode                         int
	ProviderIDs                             map[string]string
	Playlists                               map[string]int
	Seconds, Duration                       float64
	Watched, Favorite                       bool
	Updated                                 time.Time
}

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

type ProgressChange struct {
	TargetID string
	State    catalog.PlaybackState
	Expected catalog.PlaybackState
}

type ListChange struct {
	TargetID  string
	Favorite  bool
	Playlists map[string]int
}

type Plan struct {
	Summary     Summary
	Items       []Item
	Truncated   bool
	Changes     []ProgressChange
	ListChanges []ListChange
}

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

type PreviewStore struct {
	Mutex          *sync.Mutex
	Values         *map[string]Preview
	Slots          *chan struct{}
	Limit, Workers int
}

type Observation struct {
	TargetID  string `json:"targetId"`
	Signature string `json:"signature"`
}

type SyncPending struct {
	Summary     Summary          `json:"summary"`
	Changes     []ProgressChange `json:"changes,omitempty"`
	ListChanges []ListChange     `json:"listChanges,omitempty"`
}

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
