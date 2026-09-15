// Package dashboard owns the board, application, and service-health domain.
package dashboard

import "time"

const MaxApps = 200

// App is one destination on the shared board.
type App struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	HealthURL    string    `json:"healthUrl"`
	CheckEnabled bool      `json:"checkEnabled"`
	Description  string    `json:"description,omitempty"`
	Category     string    `json:"category,omitempty"`
	Icon         string    `json:"icon"`
	Accent       string    `json:"accent"`
	Favorite     bool      `json:"favorite"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Board is the single ordered application board.
type Board struct {
	Title     string       `json:"title"`
	Version   uint64       `json:"version"`
	Apps      []App        `json:"apps"`
	Removed   []RemovedApp `json:"removed,omitempty"`
	Audit     []AuditEvent `json:"audit,omitempty"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// RemovedApp preserves one recent removal for explicit recovery.
type RemovedApp struct {
	App       App       `json:"app"`
	Position  int       `json:"position"`
	RemovedAt time.Time `json:"removedAt"`
}

// AuditEvent describes one board mutation without storing credentials.
type AuditEvent struct {
	Action  string    `json:"action"`
	Subject string    `json:"subject"`
	Actor   string    `json:"actor"`
	At      time.Time `json:"at"`
}

// Health is the latest bounded reachability observation.
type Health struct {
	State       string    `json:"state"`
	LatencyMS   int64     `json:"latencyMs,omitempty"`
	HTTPStatus  int       `json:"httpStatus,omitempty"`
	CheckedAt   time.Time `json:"checkedAt,omitzero"`
	Explanation string    `json:"explanation,omitempty"`
}

// ProbeBatch reports whether a bounded multi-application check completed.
type ProbeBatch struct {
	Requested int  `json:"requested"`
	Completed int  `json:"completed"`
	Canceled  bool `json:"canceled"`
}

// AppView combines durable configuration with ephemeral health.
type AppView struct {
	App
	Health Health `json:"health"`
}

// Summary gives an honest count of current probe states.
type Summary struct {
	Total       int `json:"total"`
	Reachable   int `json:"reachable"`
	Slow        int `json:"slow"`
	Degraded    int `json:"degraded"`
	Unavailable int `json:"unavailable"`
	Unchecked   int `json:"unchecked"`
	Disabled    int `json:"disabled"`
}

// Snapshot is the API and UI representation of the board.
type Snapshot struct {
	Title     string       `json:"title"`
	Version   uint64       `json:"version"`
	Apps      []AppView    `json:"apps"`
	Removed   []RemovedApp `json:"recentlyRemoved"`
	Summary   Summary      `json:"summary"`
	Audit     []AuditEvent `json:"audit"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// Receipt is a structured record of one completed mutation.
type Receipt struct {
	Action        string `json:"action"`
	AppID         string `json:"appId,omitempty"`
	BeforeVersion uint64 `json:"beforeVersion"`
	AfterVersion  uint64 `json:"afterVersion"`
	Message       string `json:"message"`
}

// CreateInput contains all fields accepted when adding an application.
type CreateInput struct {
	CatalogID       string `json:"catalogId,omitempty"`
	Name            string `json:"name,omitempty"`
	URL             string `json:"url"`
	HealthURL       string `json:"healthUrl,omitempty"`
	CheckEnabled    bool   `json:"checkEnabled"`
	Description     string `json:"description,omitempty"`
	Category        string `json:"category,omitempty"`
	Icon            string `json:"icon,omitempty"`
	Accent          string `json:"accent,omitempty"`
	Favorite        bool   `json:"favorite,omitempty"`
	ExpectedVersion uint64 `json:"expectedVersion,omitempty"`
}

// UpdateInput uses pointers to distinguish omitted fields from empty fields.
type UpdateInput struct {
	Name            *string `json:"name,omitempty"`
	URL             *string `json:"url,omitempty"`
	HealthURL       *string `json:"healthUrl,omitempty"`
	CheckEnabled    *bool   `json:"checkEnabled,omitempty"`
	Description     *string `json:"description,omitempty"`
	Category        *string `json:"category,omitempty"`
	Icon            *string `json:"icon,omitempty"`
	Accent          *string `json:"accent,omitempty"`
	Favorite        *bool   `json:"favorite,omitempty"`
	ExpectedVersion uint64  `json:"expectedVersion,omitempty"`
}

// Import contains a complete validated board replacement.
type Import struct {
	Title           string `json:"title"`
	Apps            []App  `json:"apps"`
	ExpectedVersion uint64 `json:"expectedVersion,omitempty"`
}
