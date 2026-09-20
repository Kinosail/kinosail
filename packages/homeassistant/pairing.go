package homeassistant

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type pairing[P any] struct {
	Profile Profile[P]
	Expires time.Time
}

func (integration *Integration[P]) offer(profile Profile[P]) (string, error) {
	integration.mu.Lock()
	defer integration.mu.Unlock()
	now := integration.now()
	for code, offer := range integration.pairs {
		if !offer.Expires.After(now) {
			delete(integration.pairs, code)
		}
	}
	if len(integration.pairs) >= maxPairs {
		return "", errors.New("too many Home Assistant pairing requests")
	}
	for range 8 {
		value, err := rand.Int(integration.config.Random, big.NewInt(100000000))
		if err != nil {
			return "", err
		}
		code := fmt.Sprintf("%08d", value.Int64())
		if _, found := integration.pairs[code]; !found {
			integration.pairs[code] = pairing[P]{profile, now.Add(pairingTTL)}
			return code, nil
		}
	}
	return "", errors.New("could not create Home Assistant pairing code")
}

func (integration *Integration[P]) pair(code, name string) (string, error) {
	code, name = strings.TrimSpace(code), strings.TrimSpace(name)
	if len(code) != 8 || !allDigits(code) || name == "" || len(name) > 80 {
		return "", errors.New("Home Assistant pairing code and name are invalid") //nolint:staticcheck // Preserve the integration error.
	}
	integration.mu.Lock()
	defer integration.mu.Unlock()
	offer, found := integration.pairs[code]
	if !found || !offer.Expires.After(integration.now()) {
		delete(integration.pairs, code)
		return "", errors.New("Home Assistant pairing code is invalid or expired") //nolint:staticcheck // Preserve the integration error.
	}
	delete(integration.pairs, code)
	profile, found := integration.config.FindProfile(offer.Profile.ID)
	if !found || !profile.Owner {
		return "", errors.New("Home Assistant pairing requires a current Owner")
	}
	return integration.config.CreateKey(profile.Source, name)
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
