package supporter

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

func (service *Service) matchingActivation(state State, keyHash string) (string, bool) { //nolint:cyclop,gocognit // Every persisted reference must agree before reuse.
	matched := ""
	for _, activation := range state.Activations {
		if subtle.ConstantTimeCompare([]byte(activation.KeyHash), []byte(keyHash)) == 1 {
			if !activationIDPattern.MatchString(activation.ActivationID) || matched != "" && matched != activation.ActivationID {
				return "", false
			}
			matched = activation.ActivationID
		}
	}
	for _, grant := range []*Grant{state.PatronOrder, state.LivingStandard, legacyGrant(state)} {
		if grant != nil && subtle.ConstantTimeCompare([]byte(grant.KeyHash), []byte(keyHash)) == 1 {
			if !activationIDPattern.MatchString(grant.ActivationID) || matched != "" && matched != grant.ActivationID {
				return "", false
			}
			matched = grant.ActivationID
		}
	}
	return matched, true
}

func rememberActivation(activations [activationSlots]Activation, grant Grant) [activationSlots]Activation {
	value := Activation{ActivationID: grant.ActivationID, KeyHash: grant.KeyHash}
	for index := range activations {
		if activations[index].KeyHash == value.KeyHash || activations[index] == (Activation{}) {
			activations[index] = value
			return activations
		}
	}
	copy(activations[:], activations[1:])
	activations[len(activations)-1] = value
	return activations
}

func (service *Service) raiseLevels(state *State) {
	if decoded := service.decodeGrant(*state, state.PatronOrder, FamilyPatron, true); decoded.valid {
		state.PatronLevel = max(state.PatronLevel, Rank(decoded.certificate.Tier))
	}
	if decoded := service.decodeGrant(*state, state.LivingStandard, FamilyLiving, true); decoded.valid {
		state.LivingLevel = max(state.LivingLevel, Rank(decoded.certificate.Tier))
	}
}

// Recover removes invalid grants and retains bounded activation references.
func (service *Service) Recover(state State) (State, bool) {
	next := cloneState(state)
	changed := false
	for _, item := range []struct {
		grant  **Grant
		family string
	}{{&next.PatronOrder, FamilyPatron}, {&next.LivingStandard, FamilyLiving}} {
		if *item.grant != nil && !service.decodeGrant(next, *item.grant, item.family, true).valid {
			if validActivationReference(**item.grant) {
				next.Activations = rememberActivation(next.Activations, **item.grant)
			}
			*item.grant, changed = nil, true
		}
	}
	return next, changed
}

// RotatePublicKey clears grants only after strict explicit confirmation.
func (service *Service) RotatePublicKey(state State, input RotationInput) (State, error) {
	if input.Version != 1 || !input.Confirm {
		return state, ErrInvalid
	}
	if state.InstallationKey == "" {
		return state, ErrConflict
	}
	if _, ok := decodeValue(input.PublicKey, ed25519.PublicKeySize, 64); !ok || subtle.ConstantTimeCompare([]byte(input.PublicKey), []byte(state.PublicKey)) == 1 {
		return state, ErrInvalid
	}
	for _, grant := range []*Grant{state.PatronOrder, state.LivingStandard} {
		if grant != nil && validActivationReference(*grant) {
			state.Activations = rememberActivation(state.Activations, *grant)
		}
	}
	state.PublicKey, state.PatronOrder, state.LivingStandard = input.PublicKey, nil, nil
	return state, nil
}

// Fingerprint identifies one in-flight activation without exposing its values.
func (service *Service) Fingerprint(input ActivationInput) string {
	marker := "absent"
	if input.RecognitionName != nil {
		marker = "present:" + *input.RecognitionName
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(input.Key) + "\x00" + marker))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func validActivationReference(grant Grant) bool {
	_, hashOK := decodeValue(grant.KeyHash, 32, 64)
	return hashOK && activationIDPattern.MatchString(grant.ActivationID)
}

func emptyState(state State) bool {
	return state.InstallationKey == "" && state.PublicKey == "" && state.PatronOrder == nil && state.LivingStandard == nil &&
		state.PatronLevel == 0 && state.LivingLevel == 0 && state.Activations == ([activationSlots]Activation{}) && legacyGrant(state) == nil
}

func validLevels(state State) bool {
	return state.PatronLevel >= 0 && state.PatronLevel <= MaximumLevel && state.LivingLevel >= 0 && state.LivingLevel <= MaximumLevel
}

// ValidateState rejects malformed persisted state before network or storage side effects.
func (service *Service) ValidateState(state State) error {
	if emptyState(state) {
		return nil
	}
	if !validLevels(state) {
		return ErrInvalid
	}
	if _, ok := decodeValue(state.InstallationKey, 32, 64); !ok {
		return ErrInvalid
	}
	if state.PublicKey != "" {
		if _, ok := decodeValue(state.PublicKey, 32, 64); !ok {
			return ErrInvalid
		}
	}
	if !validStateGrants(state) || !validStateActivations(state.Activations) {
		return ErrInvalid
	}
	return nil
}

func validStateGrants(state State) bool {
	for _, grant := range []*Grant{state.PatronOrder, state.LivingStandard} {
		if grant == nil {
			continue
		}
		publicKey := grant.PublicKey
		if publicKey == "" {
			publicKey = state.PublicKey
		}
		if !activationIDPattern.MatchString(grant.ActivationID) || !validEncodedGrant(*grant) {
			return false
		}
		if _, ok := decodeValue(publicKey, ed25519.PublicKeySize, 64); !ok {
			return false
		}
	}
	return true
}

func validStateActivations(activations [activationSlots]Activation) bool {
	seen := map[string]bool{}
	for _, activation := range activations {
		if activation == (Activation{}) {
			continue
		}
		if !validActivationReference(Grant{ActivationID: activation.ActivationID, KeyHash: activation.KeyHash}) || seen[activation.KeyHash] {
			return false
		}
		seen[activation.KeyHash] = true
	}
	return true
}

func validEncodedGrant(grant Grant) bool {
	_, certificateOK := decodeValue(grant.Certificate, -1, 4096)
	_, signatureOK := decodeValue(grant.Signature, 64, 128)
	_, hashOK := decodeValue(grant.KeyHash, 32, 64)
	return certificateOK && signatureOK && hashOK
}

func cloneState(state State) State {
	state.PatronOrder, state.LivingStandard = cloneGrant(state.PatronOrder), cloneGrant(state.LivingStandard)
	return state
}
