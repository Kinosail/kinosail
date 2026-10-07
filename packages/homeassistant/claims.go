package homeassistant

import (
	"crypto/hmac"
	"errors"
	"math"
	"time"
)

var (
	errPlayerDisabled  = errors.New("not found")
	errPlayerOwnership = errors.New("Home Assistant player ownership is invalid") //nolint:staticcheck // Preserve product terminology.
	errPlayerOccupied  = errors.New("Home Assistant player is already active")    //nolint:staticcheck // Preserve product terminology.
)

type playerClaim struct {
	ID        string `json:"id"`
	Claim     string `json:"claim"`
	ExpiresIn int    `json:"expiresIn"`
}

func ownsPlayer(record playerRecord, profile, claim string, now time.Time) bool {
	return record.ID != "" && record.Claim != "" && record.Profile == profile && now.Sub(record.Seen) <= playerTTL && hmac.Equal([]byte(record.Claim), []byte(claim))
}

func validPlayerUpdateClaim(record playerRecord, profile string, claims []string, now time.Time) bool {
	if len(claims) > 1 {
		return false
	}
	claim := ""
	if len(claims) == 1 {
		claim = claims[0]
	}
	return record.Claim == "" && claim == "" || ownsPlayer(record, profile, claim, now)
}

func (integration *Integration[P]) generatedPlayerIDLocked() string {
	id := ""
	for range maxPlayers + 1 {
		id = "web-" + integration.text()
		if _, found := integration.players[id]; !found {
			break
		}
	}
	return id
}

func (integration *Integration[P]) claimPlayer(id, profile string) (playerClaim, int, error) {
	now := integration.now()
	integration.mu.Lock()
	defer integration.mu.Unlock()
	if !integration.Enabled() {
		return playerClaim{}, 0, errPlayerDisabled
	}
	integration.prunePlayersLocked(now)
	if record, found := integration.players[id]; found {
		if record.Profile != profile {
			return playerClaim{}, 0, errPlayerOwnership
		}
		retry := max(1, min(30, int(math.Ceil((playerTTL-now.Sub(record.Seen)).Seconds()))))
		return playerClaim{}, retry, errPlayerOccupied
	}
	if len(integration.players) >= maxPlayers {
		return playerClaim{}, 0, errPlayerLimit
	}
	if id == "" {
		id = integration.generatedPlayerIDLocked()
	}
	claim := integration.text()
	if !playerID.MatchString(id) || !playerID.MatchString(claim) || len(claim) < 20 {
		return playerClaim{}, 0, errors.New("player identity generation failed")
	}
	if _, found := integration.players[id]; found {
		return playerClaim{}, 0, errPlayerLimit
	}
	integration.players[id] = playerRecord{Player: Player{ID: id}, Seen: now, Profile: profile, Claim: claim}
	return playerClaim{ID: id, Claim: claim, ExpiresIn: int(playerTTL / time.Second)}, 0, nil
}

func (integration *Integration[P]) releasePlayer(id, profile, claim string) error {
	integration.mu.Lock()
	defer integration.mu.Unlock()
	if !ownsPlayer(integration.players[id], profile, claim, integration.now()) {
		return errPlayerOwnership
	}
	delete(integration.players, id)
	return nil
}
