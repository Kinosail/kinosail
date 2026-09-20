package homeassistant

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

var playerID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var errPlayerLimit = errors.New("too many active Home Assistant players")

// Player is one active Kinosail player state.
type Player struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Title    string  `json:"title,omitempty"`
	ItemID   string  `json:"itemId,omitempty"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Volume   float64 `json:"volume"`
	Muted    bool    `json:"muted"`
}

// Command is one queued player command.
type Command struct {
	Command  string  `json:"command"`
	Position float64 `json:"position,omitempty"`
	Volume   float64 `json:"volume,omitempty"`
	Muted    bool    `json:"muted,omitempty"`
	ItemID   string  `json:"itemId,omitempty"`
}

type playerRecord struct {
	Player
	Seen    time.Time
	Command *Command
	Profile string
}

func (integration *Integration[P]) playersSnapshot() []Player {
	integration.mu.Lock()
	defer integration.mu.Unlock()
	integration.prunePlayersLocked(integration.now())
	players := make([]Player, 0, len(integration.players))
	for _, record := range integration.players {
		players = append(players, record.Player)
	}
	return players
}

func (integration *Integration[P]) prunePlayersLocked(now time.Time) {
	for id, record := range integration.players {
		if now.Sub(record.Seen) > playerTTL {
			delete(integration.players, id)
		}
	}
}

func (integration *Integration[P]) updatePlayer(id string, state Player, profile string) (*Command, error) {
	state.Name = strings.TrimSpace(state.Name)
	state.ID = id
	if err := validatePlayer(state); err != nil {
		return nil, err
	}
	now := integration.now()
	integration.mu.Lock()
	defer integration.mu.Unlock()
	integration.prunePlayersLocked(now)
	record := integration.players[id]
	if record.ID != "" && record.Profile != profile {
		return nil, errors.New("Home Assistant player belongs to another Viewer Profile")
	}
	if record.ID == "" && len(integration.players) >= maxPlayers {
		return nil, errPlayerLimit
	}
	record.Player, record.Seen, record.Profile = state, now, profile
	command := record.Command
	record.Command = nil
	integration.players[id] = record
	return command, nil
}

func validatePlayer(state Player) error {
	if strings.TrimSpace(state.Name) == "" || len(state.Name) > 80 || !oneOf(state.State, "idle", "playing", "paused", "buffering") || len(state.Title) > 256 || len(state.ItemID) > 128 || invalidNumber(state.Position, 1e9) || invalidNumber(state.Duration, 1e9) || invalidNumber(state.Volume, 1) {
		return errors.New("Home Assistant player state is invalid") //nolint:staticcheck // Preserve the public error.
	}
	return nil
}

func invalidNumber(value, maximum float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maximum
}

func (integration *Integration[P]) queueCommand(id string, command Command) (string, bool, error) {
	if err := integration.validateCommand(command); err != nil {
		return "", false, err
	}
	integration.mu.Lock()
	record, found := integration.players[id]
	found = found && integration.now().Sub(record.Seen) <= playerTTL
	if found {
		record.Command = &command
		integration.players[id] = record
	} else {
		delete(integration.players, id)
	}
	publish := integration.publish
	integration.mu.Unlock()
	if found && publish != nil {
		publish(record.Profile, "home-assistant.command", "/api/v1/home-assistant/players/"+id)
	}
	return record.Profile, found, nil
}

func (integration *Integration[P]) validateCommand(command Command) error { //nolint:cyclop // One rule set validates each command type.
	if !oneOf(command.Command, "play", "pause", "stop", "seek", "volume", "mute", "play_media") || command.Command == "seek" && invalidNumber(command.Position, 1e9) || command.Command == "volume" && invalidNumber(command.Volume, 1) || command.Command == "play_media" && (command.ItemID == "" || len(command.ItemID) > 128) {
		return errors.New("Home Assistant player command is invalid") //nolint:staticcheck // Preserve the public error.
	}
	if command.Command == "play_media" {
		if _, found := integration.config.FindItem(command.ItemID); !found {
			return errors.New("Home Assistant media item was not found") //nolint:staticcheck // Preserve the public error.
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
