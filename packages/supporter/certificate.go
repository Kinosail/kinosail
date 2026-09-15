package supporter

import (
	"crypto/ed25519"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type decodedGrant struct {
	certificate Certificate
	family      string
	valid       bool
	expired     bool
}

func (service *Service) decodeGrant(state State, grant *Grant, expectedFamily string, allowLegacy bool) decodedGrant {
	record, ok := verifiedGrantRecord(state, grant)
	if !ok {
		return decodedGrant{}
	}
	certificate, version, ok := service.decodeCertificate(record, allowLegacy)
	if !ok || certificate.InstallationKey != state.InstallationKey {
		return decodedGrant{}
	}
	return service.classifyCertificate(certificate, version, expectedFamily)
}

func verifiedGrantRecord(state State, grant *Grant) ([]byte, bool) {
	if grant == nil {
		return nil, false
	}
	publicKey := state.PublicKey
	if grant.PublicKey != "" {
		publicKey = grant.PublicKey
	}
	key, keyOK := decodeValue(publicKey, ed25519.PublicKeySize, 64)
	signature, signatureOK := decodeValue(grant.Signature, ed25519.SignatureSize, 128)
	record, recordOK := decodeValue(grant.Certificate, -1, 4096)
	if !keyOK || !signatureOK || !recordOK || !utf8.Valid(record) || !validJSONUnicode(record) || !ed25519.Verify(key, record, signature) {
		return nil, false
	}
	return record, true
}

func (service *Service) decodeCertificate(record []byte, allowLegacy bool) (Certificate, int, bool) {
	var version struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(record, &version) != nil {
		return Certificate{}, 0, false
	}
	var certificate Certificate
	var valid bool
	switch version.Version {
	case 3, 4:
		valid = service.decodeCurrentCertificate(record, &certificate)
	case 2:
		valid = allowLegacy && service.app.Legacy != LegacyNone && service.decodeV2Certificate(record, &certificate)
	case 1:
		valid = allowLegacy && service.app.Legacy != LegacyNone && service.decodeV1Certificate(record, &certificate)
	default:
		valid = false
	}
	return certificate, version.Version, valid
}

func (service *Service) classifyCertificate(certificate Certificate, version int, expectedFamily string) decodedGrant { //nolint:cyclop // Certificate classification remains below the repository complexity ceiling.
	if version == 2 && certificate.Sustaining && expectedFamily == FamilyPatron {
		certificate.Sustaining, certificate.ExpiresAt, certificate.SubscriptionTier = false, "", ""
		return decodedGrant{certificate: certificate, family: FamilyPatron, valid: true}
	}
	family := FamilyPatron
	if certificate.Sustaining {
		family = FamilyLiving
	}
	if version == 2 && family == FamilyLiving && certificate.ExpiresAt == "" || expectedFamily != "" && expectedFamily != family {
		return decodedGrant{}
	}
	expired := false
	if certificate.ExpiresAt != "" {
		expiresAt, _ := time.Parse(time.RFC3339Nano, certificate.ExpiresAt)
		expired = !service.now().UTC().Before(expiresAt)
	}
	return decodedGrant{certificate: certificate, family: family, valid: true, expired: expired}
}

func (service *Service) decodeCurrentCertificate(record []byte, certificate *Certificate) bool {
	object, version, ok := parseCurrentCertificate(record, certificate)
	if !ok || !service.validCertificateIdentity(*certificate, object) || !validCollection(certificate.Collection, object["collection"], certificate.Sustaining, service.app.ID) {
		return false
	}
	return validCurrentExpiry(*certificate, object["expiresAt"]) && validCertificateTimes(*certificate, service.now().UTC(), version >= 3)
}

func parseCurrentCertificate(record []byte, certificate *Certificate) (map[string]json.RawMessage, int, bool) { //nolint:cyclop // Strict certificate validation remains below the repository complexity ceiling.
	object, err := jsonObject(record)
	required := []string{"version", "audience", "appId", "tier", "supporterId", "supportedSince", "issuedAt", "expiresAt", "sustaining", "founding", "installationKey"}
	var version int
	if json.Unmarshal(object["version"], &version) != nil || version < 3 || version > 4 {
		return nil, 0, false
	}
	if version == 4 {
		required = append(required, "family", "level")
	}
	sustaining, sustainingOK := jsonBool(object["sustaining"])
	founding, foundingOK := jsonBool(object["founding"])
	if err != nil || !exactFields(object, required, []string{"recognitionName", "collection"}) || json.Unmarshal(record, certificate) != nil ||
		!sustainingOK || !foundingOK || certificate.Sustaining != sustaining || certificate.Founding != founding || certificate.Version != version {
		return nil, 0, false
	}
	if raw, found := object["collection"]; found && string(raw) == "null" {
		return nil, 0, false
	}
	if version == 4 && (certificate.Level != Rank(certificate.Tier) || certificate.Family != map[bool]string{false: FamilyPatron, true: FamilyLiving}[certificate.Sustaining]) {
		return nil, 0, false
	}
	return object, version, true
}

func validCurrentExpiry(certificate Certificate, raw json.RawMessage) bool {
	if certificate.Sustaining {
		expiresAt, ok := jsonString(raw)
		if !ok || expiresAt == "" || certificate.ExpiresAt != expiresAt {
			return false
		}
		return true
	}
	return string(raw) == "null" && certificate.ExpiresAt == ""
}

func (service *Service) validCertificateIdentity(certificate Certificate, object map[string]json.RawMessage) bool { //nolint:cyclop // Identity invariants must be evaluated together.
	if certificate.Audience != service.app.Audience || certificate.AppID != service.app.ID || Rank(certificate.Tier) == 0 ||
		!supporterIDPattern.MatchString(certificate.SupporterID) ||
		service.app.BindSupporterIDToInstallation && certificate.SupporterID != supporterMark(certificate.InstallationKey) {
		return false
	}
	if raw, found := object["recognitionName"]; found {
		name, ok := jsonString(raw)
		if !ok || name == "" && !service.app.AllowEmptyRecognitionField || name != "" && (!ValidRecognitionName(name, true) || Rank(certificate.Tier) < 7) || name != certificate.RecognitionName {
			return false
		}
	} else if certificate.RecognitionName != "" {
		return false
	}
	return true
}

func validCollection(collection *Collection, raw json.RawMessage, sustaining bool, appID string) bool { //nolint:cyclop // Collection validation is one exact signed-data predicate.
	if collection == nil {
		return len(raw) == 0
	}
	object, err := jsonObject(raw)
	if err != nil || !exactFields(object, []string{"id", "name", "edition", "appIds"}, nil) ||
		collection.ID != completeFleetID || collection.Name != "Complete Fleet" || collection.Edition != strings.TrimSpace(collection.Edition) || !editionPattern.MatchString(collection.Edition) ||
		(collection.Edition == "Living") != sustaining || len(collection.AppIDs) < 2 || len(collection.AppIDs) > 24 || !slices.IsSorted(collection.AppIDs) {
		return false
	}
	found := false
	for index, item := range collection.AppIDs {
		if !appIDPattern.MatchString(item) || index > 0 && item == collection.AppIDs[index-1] {
			return false
		}
		found = found || item == appID
	}
	return found
}

func validCertificateTimes(certificate Certificate, now time.Time, boundExpiry bool) bool {
	supportedSince, sinceOK := strictTime(certificate.SupportedSince)
	issuedAt, issuedOK := strictTime(certificate.IssuedAt)
	if !sinceOK || !issuedOK || supportedSince.After(issuedAt) || issuedAt.After(now.Add(5*time.Minute)) {
		return false
	}
	if certificate.ExpiresAt == "" {
		return !certificate.Sustaining
	}
	expiresAt, expiryOK := strictTime(certificate.ExpiresAt)
	return expiryOK && certificate.Sustaining && expiresAt.After(issuedAt) && (!boundExpiry || !expiresAt.After(issuedAt.Add(45*24*time.Hour)))
}
