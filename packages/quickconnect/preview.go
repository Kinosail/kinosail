package quickconnect

import (
	"sort"
	"time"
)

// Preview contains only the information a Viewer needs to approve a device.
// The polling secret and session credentials never leave the requesting device.
type Preview struct {
	Code      string    `json:"code"`
	Device    string    `json:"device"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Preview returns an unapproved, local request identified by its displayed code.
func (broker *Broker) Preview(code string, viewer Viewer) (Preview, error) {
	if !numericCode(code) || !validViewer(viewer) {
		return Preview{}, ErrInvalidRequest
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	pending, found := broker.pending[broker.byCode[code]]
	if !found || pending.remote || pending.profileID != "" || !pending.expires.After(broker.now()) {
		return Preview{}, ErrInvalidCode
	}
	return preview(pending), nil
}

// PendingTVs lists at most eight local native TV requests for an authenticated Viewer.
// Other clients and remote requests remain on the explicit code approval flow.
func (broker *Broker) PendingTVs(viewer Viewer) ([]Preview, error) {
	if !validViewer(viewer) {
		return nil, ErrInvalidRequest
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	result := make([]Preview, 0)
	for _, pending := range broker.pending {
		if pending.localTV(broker.now()) {
			result = append(result, preview(pending))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ExpiresAt.Equal(result[j].ExpiresAt) {
			return result[i].Code < result[j].Code
		}
		return result[i].ExpiresAt.Before(result[j].ExpiresAt)
	})
	if len(result) > 8 {
		result = result[:8]
	}
	return result, nil
}

func preview(pending pendingConnection) Preview {
	return Preview{pending.connection.Code, pending.connection.Device, pending.expires.UTC()}
}

func validViewer(viewer Viewer) bool {
	return viewer.ID != "" && validField(viewer.ID, 128)
}

func numericCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (pending pendingConnection) localTV(now time.Time) bool {
	return !pending.remote && pending.profileID == "" && pending.expires.After(now) && pending.connection.Client == "Kinosail" && pending.connection.Device == "Kinosail TV" && numericCode(pending.connection.Code)
}
