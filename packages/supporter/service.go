package supporter

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

var audiencePattern = regexp.MustCompile(`^[a-z][A-Za-z0-9.-]{2,127}$`)

// Service applies Player's supporter policy to caller-owned state.
type Service struct {
	mu               sync.Mutex
	app              App
	endpoint         string
	supportURL       string
	trustedPublicKey string
	client           *http.Client
	now              func() time.Time
}

// New validates all external configuration before it can cause network work.
func New(config Config) (*Service, error) { //nolint:cyclop // Construction validates all adapter policy before network work.
	app := config.App
	if !appIDPattern.MatchString(app.ID) || app.Name == "" || !audiencePattern.MatchString(app.Audience) || app.MasterworkName == "" ||
		app.Legacy > LegacySubtitles ||
		config.ActivationURL != "" && !ValidEndpoint(config.ActivationURL, true) || config.SupportURL != "" && !ValidEndpoint(config.SupportURL, false) ||
		config.TrustedPublicKey != "" && !validTrustedPublicKey(config.TrustedPublicKey) {
		return nil, ErrInvalid
	}
	if app.FamilyPatron == "" {
		app.FamilyPatron = FamilyPatron
	}
	if app.FamilyLiving == "" {
		app.FamilyLiving = FamilyLiving
	}
	client := &http.Client{}
	if config.HTTPClient != nil {
		*client = *config.HTTPClient
	}
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return errors.New("redirect refused") }
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{app: app, endpoint: config.ActivationURL, supportURL: config.SupportURL, trustedPublicKey: config.TrustedPublicKey, client: client, now: config.Now}, nil
}

func validTrustedPublicKey(value string) bool {
	_, ok := decodeValue(value, 32, 64)
	return ok
}

// ActivationAvailable reports whether the user configured the remote adapter.
func (service *Service) ActivationAvailable() bool { return service.endpoint != "" }

// ValidateInput validates and normalizes no state.
func (service *Service) ValidateInput(input ActivationInput) error {
	if !keyPattern.MatchString(strings.TrimSpace(input.Key)) || input.RecognitionName != nil && !ValidRecognitionName(*input.RecognitionName, true) {
		return ErrInvalid
	}
	return nil
}

// Prepare creates one installation identity before a caller persists it.
func (service *Service) Prepare(state State) (State, error) {
	if state.InstallationKey != "" {
		if _, ok := decodeValue(state.InstallationKey, 32, 64); ok {
			return state, nil
		}
		return state, ErrInvalid
	}
	if !emptyState(state) {
		return state, ErrInvalid
	}
	value := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return state, unavailable("could not create supporter installation identity")
	}
	state.InstallationKey = base64.RawURLEncoding.EncodeToString(value)
	return state, nil
}

// Activate validates one provider response and returns the next state without persisting it.
func (service *Service) Activate(ctx context.Context, state State, input ActivationInput) (State, Status, error) {
	input.Key = strings.TrimSpace(input.Key)
	if service.ValidateInput(input) != nil {
		return state, service.Status(state), invalid("supporter key or recognition name is invalid")
	}
	if !service.ActivationAvailable() {
		return state, service.Status(state), unavailable("supporter activation is not configured")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	original := cloneState(state)
	state, activationID, keyHash, ok := service.prepareActivation(state, input.Key)
	if !ok {
		return original, service.Status(original), invalid("the saved supporter activation is invalid")
	}
	request := activationRequest{Key: input.Key, AppID: service.app.ID, InstallationKey: state.InstallationKey, ActivationID: activationID, RecognitionName: input.RecognitionName}
	output, sendErr := service.send(ctx, request)
	if sendErr != nil {
		return original, service.Status(original), sendErr
	}
	state, grant, decoded, validationErr := service.validateActivationOutput(original, state, output, activationID, keyHash, input.RecognitionName)
	if validationErr != nil {
		return original, service.Status(original), validationErr
	}
	service.raiseLevels(&state)
	return service.applyActivation(original, state, grant, decoded)
}

func (service *Service) prepareActivation(state State, key string) (State, string, string, bool) {
	state = service.migrateLegacy(cloneState(state))
	if !validLevels(state) {
		return state, "", "", false
	}
	state, err := service.Prepare(state)
	if err != nil || service.ValidateState(state) != nil {
		return state, "", "", false
	}
	keyHash := hashKey(key)
	activationID, valid := service.matchingActivation(state, keyHash)
	return state, activationID, keyHash, valid
}

func (service *Service) validateActivationOutput(original, state State, output activationResponse, activationID, keyHash string, recognitionName *string) (State, *Grant, decodedGrant, error) { //nolint:cyclop // One atomic response check must reject every invalid grant before state changes.
	if !activationIDPattern.MatchString(output.ActivationID) || activationID != "" && output.ActivationID != activationID || !acceptsPublicKey(state, output.PublicKey) ||
		service.trustedPublicKey != "" && subtle.ConstantTimeCompare([]byte(service.trustedPublicKey), []byte(output.PublicKey)) != 1 {
		return state, nil, decodedGrant{}, invalid("supporter activation returned an invalid response")
	}
	state.PublicKey = output.PublicKey
	grant := &Grant{ActivationID: output.ActivationID, Certificate: output.Certificate, Signature: output.Signature, KeyHash: keyHash}
	if service.app.GrantPublicKey {
		state.PublicKey, grant.PublicKey = original.PublicKey, output.PublicKey
	}
	decoded := service.decodeGrant(state, grant, "", false)
	if !decoded.valid || decoded.expired || decoded.certificate.InstallationKey != state.InstallationKey || !matchesRecognition(decoded.certificate, recognitionName) {
		return state, nil, decodedGrant{}, invalid("supporter activation returned an invalid certificate")
	}
	return state, grant, decoded, nil
}

func (service *Service) applyActivation(original, state State, grant *Grant, decoded decodedGrant) (State, Status, error) { //nolint:gocognit // Activation precedence remains below the repository complexity ceiling.
	rank := Rank(decoded.certificate.Tier)
	if decoded.family == FamilyPatron {
		if current := service.decodeGrant(state, state.PatronOrder, FamilyPatron, true); current.valid {
			currentRank := Rank(current.certificate.Tier)
			if currentRank > rank || currentRank == rank && current.certificate.Collection != nil && decoded.certificate.Collection == nil {
				if service.app.RejectPatronDowngrade {
					if service.app.TrackActivations {
						state.Activations = rememberActivation(state.Activations, *grant)
					}
					return state, service.Status(original), ErrConflict
				}
				return original, service.Status(original), nil
			}
		}
		state.PatronOrder, state.PatronLevel = grant, max(state.PatronLevel, rank)
	} else {
		applyRecurring(&state, grant, decoded.certificate)
	}

	if service.app.TrackActivations {
		state.Activations = rememberActivation(state.Activations, *grant)
	}
	return state, service.Status(state), nil
}

// ActivateAndSave commits a changed activation through one caller-owned persistence boundary.
func (service *Service) ActivateAndSave(ctx context.Context, state State, input ActivationInput, save func(State) error) (State, Status, error) {
	next, status, err := service.Activate(ctx, state, input)
	if err != nil || reflect.DeepEqual(state, next) {
		return state, status, err
	}
	if save == nil || save(next) != nil {
		return state, service.Status(state), unavailable("supporter activation succeeded but could not be saved")
	}
	return next, status, nil
}

func matchesRecognition(certificate Certificate, submitted *string) bool {
	if submitted == nil || Rank(certificate.Tier) < 7 {
		return certificate.RecognitionName == ""
	}
	return certificate.RecognitionName == *submitted
}

func acceptsPublicKey(state State, publicKey string) bool {
	for _, saved := range []string{state.PublicKey, grantKey(state.PatronOrder), grantKey(state.LivingStandard), grantKey(state.Monthly), grantKey(state.Yearly)} {
		if saved != "" && subtle.ConstantTimeCompare([]byte(saved), []byte(publicKey)) != 1 {
			return false
		}
	}
	return true
}

func grantKey(grant *Grant) string {
	if grant == nil {
		return ""
	}
	return grant.PublicKey
}

func invalid(message string) error     { return wrappedError{ErrInvalid, message} }
func unavailable(message string) error { return wrappedError{ErrUnavailable, message} }
func uncertain(message string) error   { return wrappedError{ErrUncertain, message} }

type wrappedError struct {
	cause   error
	message string
}

func (err wrappedError) Error() string { return err.message }
func (err wrappedError) Unwrap() error { return err.cause }
