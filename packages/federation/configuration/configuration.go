package federationconfig

import (
	"errors"
	"net/http"
	"path/filepath"
	"sync"

	federation "github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	// OIDCConfigurationKey identifies the complete OpenID Connect settings group.
	OIDCConfigurationKey = "integrations.oidc"
	OIDCIssuerKey        = federation.OIDCIssuerKey
	OIDCClientIDKey      = federation.OIDCClientIDKey
	OIDCSecretKey        = federation.OIDCSecretKey
	OIDCRedirectKey      = federation.OIDCRedirectKey
	OIDCIdentityKey      = federation.OIDCIdentityKey
	// SAMLConfigurationKey identifies the complete SAML settings group.
	SAMLConfigurationKey = "integrations.saml"
	SAMLMetadataURLKey   = federation.SAMLMetadataURLKey
	SAMLMetadataXMLKey   = federation.SAMLMetadataXMLKey
	SAMLIdentityKey      = federation.SAMLIdentityKey
)

// OIDCConfig is the shared federation configuration value.
type OIDCConfig = federation.OIDCConfig

// SAMLConfig is the shared federation configuration value.
type SAMLConfig = federation.SAMLConfig

var (
	oidcSettingKeys = []string{OIDCIssuerKey, OIDCClientIDKey, OIDCSecretKey, OIDCRedirectKey, OIDCIdentityKey}
	samlSettingKeys = []string{SAMLMetadataURLKey, SAMLMetadataXMLKey, SAMLIdentityKey}
)

// OIDCConfigurationKeys returns each setting managed as one OpenID Connect configuration.
func OIDCConfigurationKeys() []string { return append([]string(nil), oidcSettingKeys...) }

// SAMLConfigurationKeys returns each setting managed as one SAML configuration.
func SAMLConfigurationKeys() []string { return append([]string(nil), samlSettingKeys...) }

// SAMLConfigured reports whether either supported metadata source is present.
func SAMLConfigured(value func(string) string) bool {
	return value(SAMLMetadataURLKey) != "" || value(SAMLMetadataXMLKey) != ""
}

// OIDCConfigurationView contains the shared OpenID Connect setup presentation state.
type OIDCConfigurationView[C any] struct {
	Issuer, ClientID, RedirectURL, IdentityClaim string
	Configured, SecretConfigured                 bool
	Control                                      C
}

// NewOIDCConfigurationView derives the complete OpenID Connect setup state.
func NewOIDCConfigurationView[C any](value func(string) (string, bool), control C) OIDCConfigurationView[C] {
	issuer, configured := value(OIDCIssuerKey)
	_, secretConfigured := value(OIDCSecretKey)
	clientID, _ := value(OIDCClientIDKey)
	redirectURL, _ := value(OIDCRedirectKey)
	identityClaim, _ := value(OIDCIdentityKey)
	return OIDCConfigurationView[C]{issuer, clientID, redirectURL, identityClaim, configured, secretConfigured, control}
}

// OIDCConfigurationStore adapts app storage to one atomic OpenID Connect settings operation.
type OIDCConfigurationStore[S ~string] struct {
	File    string
	Lock    sync.Locker
	Current func(string) string
	Managed func(string) bool
	Source  func(string) S
	Set     func(string, string, string, string, string, string) error
	Delete  func(string) error
	Update  func(string, string, bool)
}

// NewOIDCConfigurationStore adapts one app's settings storage callbacks.
func NewOIDCConfigurationStore[S ~string](file string, lock sync.Locker, current func(string) string, managed func(string) bool, source func(string) S, set func(string, string, string, string, string, string) error, deleteConfiguration func(string) error, update func(string, string, bool)) OIDCConfigurationStore[S] {
	return OIDCConfigurationStore[S]{File: file, Lock: lock, Current: current, Managed: managed, Source: source, Set: set, Delete: deleteConfiguration, Update: update}
}

// Change validates ownership before it persists and projects one OpenID Connect settings change.
func (store OIDCConfigurationStore[S]) Change(config OIDCConfig, reset bool) error {
	store.Lock.Lock()
	defer store.Lock.Unlock()
	if store.File == "" {
		return errors.New("configuration storage is unavailable")
	}
	for _, key := range oidcSettingKeys {
		if store.Managed(key) {
			return errors.New(key + " is managed by " + string(store.Source(key)))
		}
	}
	directory := filepath.Dir(store.File)
	if reset {
		if err := store.Delete(directory); err != nil {
			return err
		}
		for _, key := range oidcSettingKeys {
			store.Update(key, "", true)
		}
		return nil
	}
	if config.ClientSecret == "" {
		config.ClientSecret = store.Current(OIDCSecretKey)
	}
	if err := store.Set(directory, config.Issuer, config.ClientID, config.ClientSecret, config.RedirectURL, config.IdentityClaim); err != nil {
		return err
	}
	values := map[string]string{OIDCIssuerKey: config.Issuer, OIDCClientIDKey: config.ClientID, OIDCSecretKey: config.ClientSecret, OIDCRedirectKey: config.RedirectURL, OIDCIdentityKey: config.IdentityClaim}
	for key, value := range values {
		store.Update(key, value, false)
	}
	return nil
}

// ParseOIDCConfigurationForm decodes one bounded, unambiguous OpenID Connect settings form.
func ParseOIDCConfigurationForm(writer http.ResponseWriter, request *http.Request) (OIDCConfig, error) {
	keys := []string{"key", "issuer", "clientId", "clientSecret", "redirectUrl", "identityClaim"}
	if err := httpguard.DecodeForm(writer, request, 16<<10, keys...); err != nil || request.PostForm.Get("key") != OIDCConfigurationKey {
		return OIDCConfig{}, errors.New("invalid configuration form")
	}
	issuer, issuerOK := oidcFormField(request, "issuer", 2048, true)
	clientID, clientIDOK := oidcFormField(request, "clientId", 512, true)
	secret, secretOK := oidcFormField(request, "clientSecret", 4096, false)
	redirectURL, redirectOK := oidcFormField(request, "redirectUrl", 2048, true)
	identityClaim, identityOK := oidcFormField(request, "identityClaim", 256, true)
	if !issuerOK || !clientIDOK || !secretOK || !redirectOK || !identityOK {
		return OIDCConfig{}, errors.New("invalid configuration form")
	}
	return OIDCConfig{Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, IdentityClaim: identityClaim}, nil
}

func oidcFormField(request *http.Request, key string, maximum int, required bool) (string, bool) {
	values := request.PostForm[key]
	if len(values) != 1 || len(values[0]) > maximum || required && values[0] == "" {
		return "", false
	}
	return values[0], true
}

// SaveOIDCConfiguration handles one shared form and delegates its durable change.
func SaveOIDCConfiguration(writer http.ResponseWriter, request *http.Request, change func(string, string, string, string, string, bool) error, writeError func(http.ResponseWriter, *http.Request, string, int)) {
	config, err := ParseOIDCConfigurationForm(writer, request)
	if err != nil {
		writeError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err = change(config.Issuer, config.ClientID, config.ClientSecret, config.RedirectURL, config.IdentityClaim, false); err != nil {
		writeError(writer, request, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(writer, request, "/settings/configuration#integrations.oidc", http.StatusSeeOther)
}

// SAMLConfigurationView contains the shared SAML setup presentation state.
type SAMLConfigurationView[C any] struct {
	ProviderMetadataURL, ProviderMetadataXML, SPMetadataURL, ACSURL, IdentityAttribute string
	Configured                                                                         bool
	Control                                                                            C
}

// NewSAMLConfigurationView derives the complete SAML setup state.
func NewSAMLConfigurationView[C any](value func(string) (string, bool), origin string, control C) SAMLConfigurationView[C] {
	metadataURL, urlConfigured := value(SAMLMetadataURLKey)
	metadataXML, xmlConfigured := value(SAMLMetadataXMLKey)
	identityAttribute, _ := value(SAMLIdentityKey)
	return SAMLConfigurationView[C]{metadataURL, metadataXML, origin + "/login/saml/metadata", origin + "/login/saml/acs", identityAttribute, urlConfigured || xmlConfigured, control}
}

// SAMLConfigurationStore adapts app storage to one atomic SAML settings operation.
type SAMLConfigurationStore[S ~string] struct {
	File    string
	Lock    sync.Locker
	Managed func(string) bool
	Source  func(string) S
	Set     func(string, string, string, string) error
	Delete  func(string) error
	Update  func(string, string, bool)
}

// Change validates ownership before it persists and projects one SAML change.
func (store SAMLConfigurationStore[S]) Change(config federation.SAMLConfig, reset bool) error {
	store.Lock.Lock()
	defer store.Lock.Unlock()
	if store.File == "" {
		return errors.New("configuration storage is unavailable")
	}
	for _, key := range samlSettingKeys {
		if store.Managed(key) {
			return errors.New(key + " is managed by " + string(store.Source(key)))
		}
	}
	directory := filepath.Dir(store.File)
	if reset {
		if err := store.Delete(directory); err != nil {
			return err
		}
	} else if err := store.Set(directory, config.MetadataURL, config.MetadataXML, config.IdentityAttribute); err != nil {
		return err
	}
	for _, key := range samlSettingKeys {
		store.Update(key, "", true)
	}
	if !reset {
		store.Update(SAMLMetadataURLKey, config.MetadataURL, false)
		store.Update(SAMLMetadataXMLKey, config.MetadataXML, false)
		store.Update(SAMLIdentityKey, config.IdentityAttribute, false)
	}
	return nil
}

// ParseSAMLConfigurationForm decodes one bounded, unambiguous SAML settings form.
func ParseSAMLConfigurationForm(writer http.ResponseWriter, request *http.Request) (federation.SAMLConfig, error) {
	keys := []string{"key", "metadataUrl", "metadataXml", "identityAttribute"}
	if err := httpguard.DecodeForm(writer, request, 1<<20, keys...); err != nil || request.PostForm.Get("key") != SAMLConfigurationKey {
		return federation.SAMLConfig{}, errors.New("invalid configuration form")
	}
	metadataURL, urlOK := boundedFormField(request, "metadataUrl", 2048, false)
	metadataXML, xmlOK := boundedFormField(request, "metadataXml", 256<<10, false)
	identityAttribute, identityOK := boundedFormField(request, "identityAttribute", 256, true)
	if !urlOK || !xmlOK || !identityOK {
		return federation.SAMLConfig{}, errors.New("invalid configuration form")
	}
	return federation.SAMLConfig{MetadataURL: metadataURL, MetadataXML: metadataXML, IdentityAttribute: identityAttribute}, nil
}

func boundedFormField(request *http.Request, key string, maximum int, required bool) (string, bool) {
	values := request.PostForm[key]
	if len(values) != 1 || len(values[0]) > maximum || required && values[0] == "" {
		return "", false
	}
	return values[0], true
}

// SaveSAMLConfiguration handles one shared form and delegates its durable change.
func SaveSAMLConfiguration(writer http.ResponseWriter, request *http.Request, change func(string, string, string, bool) error, writeError func(http.ResponseWriter, *http.Request, string, int)) {
	config, err := ParseSAMLConfigurationForm(writer, request)
	if err != nil {
		writeError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err = change(config.MetadataURL, config.MetadataXML, config.IdentityAttribute, false); err != nil {
		writeError(writer, request, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(writer, request, "/settings/configuration#integrations.saml", http.StatusSeeOther)
}
