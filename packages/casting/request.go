// Package casting owns validated TV playback requests and local renderer control.
package casting

import (
	"errors"
	"math"
	"regexp"
)

var (
	ErrInvalid = errors.New("TV playback request is invalid")
	identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	sessionID  = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

// Start names a protocol, never an arbitrary network address or media URL.
type Start struct {
	Protocol      string  `json:"protocol"`
	DeviceID      string  `json:"deviceId,omitempty"`
	Position      float64 `json:"position"`
	PlaybackToken string  `json:"playbackToken,omitempty"`
}

func (input Start) Validate(itemID string) error {
	if !identifier.MatchString(itemID) || !Position(input.Position) || len(input.PlaybackToken) > 8192 {
		return ErrInvalid
	}
	switch input.Protocol {
	case "google-cast":
		if input.DeviceID != "" {
			return ErrInvalid
		}
	case "dlna":
		if !sessionID.MatchString(input.DeviceID) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func Position(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 31_536_000
}
func ValidSessionID(value string) bool { return sessionID.MatchString(value) }

type Command struct {
	Action   string   `json:"action"`
	Position *float64 `json:"position,omitempty"`
}

func (command Command) Validate() error {
	switch command.Action {
	case "play", "pause", "stop":
		if command.Position != nil {
			return ErrInvalid
		}
	case "seek":
		if command.Position == nil || !Position(*command.Position) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
