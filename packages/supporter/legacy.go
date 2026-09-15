package supporter

import (
	"encoding/json"
	"time"
)

func (service *Service) decodeV1Certificate(record []byte, certificate *Certificate) bool { //nolint:cyclop // Strict legacy validation remains below the repository complexity ceiling.
	object, err := jsonObject(record)
	required := []string{"version", "tier", "supporterId", "supportedSince", "issuedAt", "expiresAt", "sustaining", "founding", "installationKey"}
	sustaining, sustainingOK := jsonBool(object["sustaining"])
	founding, foundingOK := jsonBool(object["founding"])
	if err != nil || !exactFields(object, required, []string{"recognitionName"}) || json.Unmarshal(record, certificate) != nil ||
		!sustainingOK || !foundingOK || certificate.Version != 1 || certificate.Sustaining != sustaining || certificate.Founding != founding ||
		Rank(certificate.Tier) == 0 || !supporterIDPattern.MatchString(certificate.SupporterID) || !service.validLegacyRecognition(object, *certificate, false) ||
		!validLegacyExpiryField(object["expiresAt"], certificate.ExpiresAt) {
		return false
	}
	if !service.validV1Times(*certificate) {
		return false
	}
	certificate.AppID, certificate.Audience, certificate.RecognitionName = service.app.ID, service.app.Audience, ""
	return true
}

func (service *Service) validV1Times(certificate Certificate) bool {
	supportedSince, sinceErr := time.Parse(time.RFC3339Nano, certificate.SupportedSince)
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, certificate.IssuedAt)
	if sinceErr != nil || issuedErr != nil || supportedSince.After(issuedAt) || issuedAt.After(service.now().UTC().Add(5*time.Minute)) || certificate.Sustaining != (certificate.ExpiresAt != "") {
		return false
	}
	if certificate.ExpiresAt != "" {
		expiresAt, expiresErr := time.Parse(time.RFC3339Nano, certificate.ExpiresAt)
		if expiresErr != nil || !expiresAt.After(issuedAt) || expiresAt.After(issuedAt.Add(45*24*time.Hour)) {
			return false
		}
	}
	return true
}

func (service *Service) decodeV2Certificate(record []byte, certificate *Certificate) bool { //nolint:cyclop // Strict legacy validation remains below the repository complexity ceiling.
	object, err := jsonObject(record)
	required := []string{"version", "audience", "tier", "supporterId", "supportedSince", "issuedAt", "expiresAt", "sustaining", "founding", "installationKey"}
	optional := []string{"recognitionName"}
	if service.app.Legacy == LegacyPlayer {
		optional = append(optional, "subscriptionTier")
	}
	sustaining, sustainingOK := jsonBool(object["sustaining"])
	founding, foundingOK := jsonBool(object["founding"])
	if err != nil || !exactFields(object, required, optional) || json.Unmarshal(record, certificate) != nil || !sustainingOK || !foundingOK ||
		certificate.Version != 2 || certificate.Audience != service.app.Audience || certificate.Sustaining != sustaining || certificate.Founding != founding ||
		Rank(certificate.Tier) == 0 || !supporterIDPattern.MatchString(certificate.SupporterID) || !service.validLegacyRecognition(object, *certificate, service.app.Legacy == LegacyPlayer) ||
		!validLegacyExpiryField(object["expiresAt"], certificate.ExpiresAt) {
		return false
	}
	if service.app.Legacy == LegacyPlayer && !validLegacySubscription(object, certificate) {
		return false
	}
	certificate.AppID, certificate.RecognitionName = service.app.ID, ""
	return service.validV2Times(*certificate)
}

func validLegacySubscription(object map[string]json.RawMessage, certificate *Certificate) bool {
	if raw, found := object["subscriptionTier"]; found {
		value, ok := jsonString(raw)
		if !ok || value != certificate.SubscriptionTier {
			return false
		}
	}
	if certificate.SubscriptionTier != "" && (!certificate.Sustaining || subscriptionName(certificate.SubscriptionTier) == "") {
		return false
	}
	if certificate.Sustaining && certificate.SubscriptionTier == "" {
		certificate.SubscriptionTier = "watch"
	}
	return true
}

func (service *Service) validLegacyRecognition(object map[string]json.RawMessage, certificate Certificate, restrictTier bool) bool {
	raw, found := object["recognitionName"]
	if !found {
		return certificate.RecognitionName == ""
	}
	name, ok := jsonString(raw)
	if !ok || name != certificate.RecognitionName || !ValidRecognitionName(name, service.app.Legacy == LegacySubtitles) {
		return false
	}
	return !restrictTier || name == "" || Rank(certificate.Tier) >= 7
}

func (service *Service) validV2Times(certificate Certificate) bool { //nolint:cyclop // Two historical time profiles must remain visibly distinct and fail closed.
	parse := func(value string) (time.Time, bool) {
		if service.app.Legacy == LegacySubtitles {
			return strictTime(value)
		}
		valueTime, err := time.Parse(time.RFC3339Nano, value)
		return valueTime, err == nil
	}
	supportedSince, sinceOK := parse(certificate.SupportedSince)
	issuedAt, issuedOK := parse(certificate.IssuedAt)
	if !sinceOK || !issuedOK || supportedSince.After(issuedAt) || issuedAt.After(service.now().UTC().Add(5*time.Minute)) || !certificate.Sustaining && certificate.ExpiresAt != "" {
		return false
	}
	if certificate.ExpiresAt == "" {
		return !certificate.Sustaining || service.app.Legacy == LegacyPlayer && certificate.SubscriptionTier == "watch"
	}
	expiresAt, expiryOK := parse(certificate.ExpiresAt)
	if service.app.Legacy == LegacyPlayer {
		return certificate.Sustaining && expiryOK && !expiresAt.Before(issuedAt)
	}
	return certificate.Sustaining && expiryOK && expiresAt.After(issuedAt)
}

func validLegacyExpiryField(raw json.RawMessage, value string) bool {
	if string(raw) == "null" {
		return value == ""
	}
	expiresAt, ok := jsonString(raw)
	return ok && expiresAt != "" && expiresAt == value
}

func legacyGrant(state State) *Grant {
	if state.Certificate == "" && state.Signature == "" && state.ActivationID == "" && state.KeyHash == "" {
		return nil
	}
	return &Grant{ActivationID: state.ActivationID, Certificate: state.Certificate, Signature: state.Signature, KeyHash: state.KeyHash}
}

func (service *Service) migrateLegacy(state State) State {
	grant := legacyGrant(state)
	if grant == nil {
		return state
	}
	if service.app.GrantPublicKey {
		grant.PublicKey = state.PublicKey
	}
	migrated := false
	if decoded := service.decodeGrant(state, grant, FamilyPatron, true); decoded.valid && state.PatronOrder == nil {
		state.PatronOrder, migrated = cloneGrant(grant), true
	}
	if decoded := service.decodeGrant(state, grant, FamilyLiving, true); decoded.valid && state.LivingStandard == nil {
		state.LivingStandard, migrated = cloneGrant(grant), true
	}
	if migrated {
		state.ActivationID, state.Certificate, state.Signature, state.KeyHash = "", "", "", ""
		if service.app.GrantPublicKey {
			state.PublicKey = ""
		}
	}
	return state
}

func cloneGrant(grant *Grant) *Grant {
	if grant == nil {
		return nil
	}
	cloned := *grant
	return &cloned
}
