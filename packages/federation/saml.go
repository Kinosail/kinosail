package federation

import (
	"context"
	"crypto/rand"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	dsig "github.com/russellhaering/goxmldsig"
)

const (
	maxSAMLBody             = 1 << 20
	maxSAMLMetadata         = 256 << 10
	metadataRefreshInterval = time.Hour
)

// SAMLConfig configures SAML 2.0 Web Browser SSO.
type SAMLConfig struct{ MetadataURL, MetadataXML, IdentityAttribute, RootURL, DataDir string }

type samlTransaction struct {
	relayState  string
	profileID   string
	linkSession string
	expires     time.Time
}

// SAML owns service-provider keys, provider metadata, request state, and assertion validation.
type SAML struct {
	config           SAMLConfig
	mu               sync.Mutex
	sp               *saml.ServiceProvider
	metadataLoadedAt time.Time
	pending          map[string]samlTransaction
}

// SAMLStart contains the validated provider response for one authentication request.
type SAMLStart struct {
	IDPURL, RedirectURL string
	PostHTML            []byte
}

// SAMLCallback is the verified result of one assertion response.
type SAMLCallback struct {
	Identity    Identity
	ProfileID   string
	LinkSession string
}

// NewSAML creates an isolated SAML flow.
func NewSAML(config SAMLConfig) *SAML {
	if config.IdentityAttribute == "" {
		config.IdentityAttribute = "NameID"
	}
	return &SAML{config: config, pending: make(map[string]samlTransaction)}
}

// Configured reports whether exactly one trusted metadata source and one valid root URL exist.
func (flow *SAML) Configured() bool {
	if flow == nil || !ValidIdentityField(flow.config.IdentityAttribute) || !validDataDirectory(flow.config.DataDir) {
		return false
	}
	metadata, err := url.Parse(flow.config.MetadataURL)
	metadataURLConfigured := boundedURLText(flow.config.MetadataURL) && err == nil && TrustedURL(metadata)
	metadataXMLConfigured := validInlineMetadata(flow.config.MetadataXML)
	return metadataURLConfigured != metadataXMLConfigured && flow.rootConfigured()
}

// Begin creates one bounded request after it validates provider metadata and its destination.
func (flow *SAML) Begin(ctx context.Context, profileID, linkSession string) (SAMLStart, error) {
	sp, err := flow.serviceProvider(ctx, true, true)
	if err != nil {
		return SAMLStart{}, err
	}
	binding, idpURL, err := samlProviderEndpoint(sp.IDPMetadata)
	if err != nil {
		return SAMLStart{}, err
	}
	authentication, err := sp.MakeAuthenticationRequest(idpURL, binding, saml.HTTPPostBinding)
	if err != nil {
		return SAMLStart{}, ErrProviderUnavailable
	}
	now := time.Now()
	flow.mu.Lock()
	prune(flow.pending, now, func(transaction samlTransaction) time.Time { return transaction.expires })
	if len(flow.pending) >= maxPendingStates {
		flow.mu.Unlock()
		return SAMLStart{}, ErrTooManyPending
	}
	relayState := rand.Text()
	flow.pending[authentication.ID] = samlTransaction{relayState: relayState, profileID: profileID, linkSession: linkSession, expires: now.Add(transactionTTL)}
	flow.mu.Unlock()
	start := SAMLStart{IDPURL: idpURL}
	if binding == saml.HTTPPostBinding {
		start.PostHTML = authentication.Post(relayState)
		return start, nil
	}
	destination, err := authentication.Redirect(relayState, sp)
	if err != nil {
		return SAMLStart{}, ErrProviderUnavailable
	}
	start.RedirectURL = destination.String()
	return start, nil
}

// Complete rejects malformed input before provider work, then validates and consumes one assertion.
func (flow *SAML) Complete(writer http.ResponseWriter, request *http.Request) (SAMLCallback, error) { //nolint:cyclop // Protocol checks must remain in this order.
	request.Body = http.MaxBytesReader(writer, request.Body, maxSAMLBody)
	if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !validSAMLForm(request.PostForm) {
		return SAMLCallback{}, ErrInvalidCallback
	}
	sp, err := flow.serviceProvider(request.Context(), true, false)
	if err != nil {
		return SAMLCallback{}, ErrProviderUnavailable
	}
	flow.mu.Lock()
	now := time.Now()
	prune(flow.pending, now, func(transaction samlTransaction) time.Time { return transaction.expires })
	requestIDs := make([]string, 0, len(flow.pending))
	for id, pending := range flow.pending {
		if pending.relayState != "" && pending.relayState == request.PostForm.Get("RelayState") {
			requestIDs = append(requestIDs, id)
		}
	}
	flow.mu.Unlock()
	if len(requestIDs) == 0 {
		return SAMLCallback{}, ErrInvalidState
	}
	assertion, err := sp.ParseResponse(request, requestIDs)
	if err != nil || assertion == nil || assertion.Subject == nil || assertion.Subject.NameID == nil || assertion.Issuer.Value != sp.IDPMetadata.EntityID {
		return SAMLCallback{}, ErrTokenInvalid
	}
	subject, err := samlIdentityValue(assertion, flow.config.IdentityAttribute)
	if err != nil {
		return SAMLCallback{}, ErrTokenInvalid
	}
	transaction, found := flow.takeTransaction(samlAssertionRequestID(assertion))
	return samlCallbackResult(assertion, subject, transaction, found)
}

func samlCallbackResult(assertion *saml.Assertion, subject string, transaction samlTransaction, found bool) (SAMLCallback, error) {
	if !found {
		return SAMLCallback{}, ErrInvalidState
	}
	return SAMLCallback{Identity: Identity{Issuer: assertion.Issuer.Value, Subject: subject}, ProfileID: transaction.profileID, LinkSession: transaction.linkSession}, nil
}

func (flow *SAML) takeTransaction(requestID string) (samlTransaction, bool) {
	flow.mu.Lock()
	defer flow.mu.Unlock()
	transaction, found := flow.pending[requestID]
	if found {
		delete(flow.pending, requestID)
	}
	return transaction, found && time.Now().Before(transaction.expires)
}

// Metadata returns this service provider's XML metadata.
func (flow *SAML) Metadata(ctx context.Context) ([]byte, error) {
	sp, err := flow.serviceProvider(ctx, false, false)
	if err != nil {
		return nil, err
	}
	return marshalSAMLMetadata(sp.Metadata())
}

func marshalSAMLMetadata(metadata *saml.EntityDescriptor) ([]byte, error) {
	return marshalSAMLMetadataWith(metadata, func(document *saml.EntityDescriptor) ([]byte, error) {
		return xml.MarshalIndent(document, "", "  ")
	})
}

func marshalSAMLMetadataWith(metadata *saml.EntityDescriptor, marshal func(*saml.EntityDescriptor) ([]byte, error)) ([]byte, error) {
	data, err := marshal(metadata)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

func (flow *SAML) rootConfigured() bool {
	if len(flow.config.RootURL) > 2048 {
		return false
	}
	root, err := url.Parse(flow.config.RootURL)
	return err == nil && TrustedURL(root) && (root.Path == "" || root.Path == "/")
}

func (flow *SAML) serviceProvider(ctx context.Context, withIDP, refresh bool) (*saml.ServiceProvider, error) { //nolint:cyclop,gocognit // Cache and metadata transitions share one lock.
	flow.mu.Lock()
	defer flow.mu.Unlock()
	if !flow.rootConfigured() || !validDataDirectory(flow.config.DataDir) || withIDP && !flow.Configured() {
		return nil, ErrNotConfigured
	}
	var loadedMetadata *saml.EntityDescriptor
	if withIDP && (flow.sp == nil || flow.sp.IDPMetadata == nil || refresh && flow.config.MetadataURL != "" && time.Since(flow.metadataLoadedAt) >= metadataRefreshInterval) {
		metadata, err := flow.loadMetadata(ctx)
		if err != nil {
			if flow.sp != nil && flow.sp.IDPMetadata != nil {
				return flow.sp, nil
			}
			return nil, err
		}
		loadedMetadata = metadata
	}
	if flow.sp == nil {
		root, _ := url.Parse(flow.config.RootURL)
		key, certificate, err := loadOrCreateSAMLKeyPair(flow.config.DataDir)
		if err != nil {
			return nil, err
		}
		metadataURL, _ := root.Parse("/login/saml/metadata")
		acsURL, _ := root.Parse("/login/saml/acs")
		flow.sp = &saml.ServiceProvider{EntityID: metadataURL.String(), Key: key, Certificate: certificate, MetadataURL: *metadataURL, AcsURL: *acsURL, AuthnNameIDFormat: saml.UnspecifiedNameIDFormat, SignatureMethod: dsig.RSASHA256SignatureMethod, HTTPClient: samlHTTPClient()}
	}
	if loadedMetadata != nil {
		updated := *flow.sp
		updated.IDPMetadata = loadedMetadata
		flow.sp = &updated
		flow.metadataLoadedAt = time.Now()
	}
	return flow.sp, nil
}

func validInlineMetadata(document string) bool {
	if document == "" {
		return false
	}
	if len(document) > maxSAMLMetadata || !utf8.ValidString(document) {
		return false
	}
	return ValidSAMLMetadata(document)
}

func validDataDirectory(path string) bool {
	return len(path) <= 4096 && utf8.ValidString(path) && !strings.ContainsRune(path, 0)
}

func (flow *SAML) loadMetadata(ctx context.Context) (*saml.EntityDescriptor, error) {
	var metadata *saml.EntityDescriptor
	var err error
	if flow.config.MetadataXML != "" {
		metadata, err = samlsp.ParseMetadata([]byte(flow.config.MetadataXML))
	} else {
		metadata, err = fetchSAMLMetadata(ctx, flow.config.MetadataURL)
	}
	if err == nil {
		_, _, err = samlProviderEndpoint(metadata)
	}
	return metadata, err
}
