package homeassistant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	authorizationTTL time.Duration = 300_000_000_000
	codeTTL                        = time.Minute
	oauthLimit                     = 32
	clientID                       = "home-assistant"
)

type authorization struct {
	RedirectURI string
	State       string
	Challenge   string
	ProfileID   string
	Expires     time.Time
}

type tokenRequest struct{ Code, RedirectURI, Verifier string }

func (integration *Integration[P]) authorizeHTTP(writer http.ResponseWriter, request *http.Request, approval func(http.ResponseWriter, *http.Request, Approval) error) {
	if request.Method == http.MethodPost {
		integration.approveAuthorization(writer, request)
		return
	}
	profile := integration.config.CurrentProfile(request)
	transaction, err := integration.authorizationRequest(request, profile)
	if err != nil {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	requestID := integration.text()
	integration.mu.Lock()
	integration.pruneOAuthLocked(integration.now())
	if len(integration.requests) >= oauthLimit {
		integration.mu.Unlock()
		oauthError(writer, "temporarily_unavailable", http.StatusTooManyRequests)
		return
	}
	integration.requests[secretHash(requestID)] = transaction
	integration.mu.Unlock()
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	destination, _ := url.Parse(transaction.RedirectURI)
	_ = approval(writer, request, Approval{profile.Name, requestID, destination.Host})
}

func (integration *Integration[P]) authorizationRequest(request *http.Request, profile Profile[P]) (authorization, error) { //nolint:cyclop // OAuth request validation remains explicit and fail-closed.
	query := request.URL.Query()
	if !onlyValues(query, "response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method") {
		return authorization{}, errors.New("invalid authorization request")
	}
	responseType, responseOK := oneValue(query, "response_type", 16)
	requestedClient, clientOK := oneValue(query, "client_id", 32)
	redirectURI, redirectOK := oneValue(query, "redirect_uri", 2048)
	state, stateOK := oneValue(query, "state", 2048)
	challenge, challengeOK := oneValue(query, "code_challenge", 128)
	method, methodOK := oneValue(query, "code_challenge_method", 8)
	if !responseOK || !clientOK || !redirectOK || !stateOK || !challengeOK || !methodOK || responseType != "code" || requestedClient != clientID || method != "S256" || !validPKCEValue(challenge) || !validRedirect(redirectURI) || !profile.Owner {
		return authorization{}, errors.New("invalid authorization request")
	}
	return authorization{redirectURI, state, challenge, profile.ID, integration.now().Add(authorizationTTL)}, nil
}

func validRedirect(raw string) bool {
	if raw == "https://my.home-assistant.io/redirect/oauth" {
		return true
	}
	parsed, err := url.Parse(raw)
	return err == nil && len(raw) <= 2048 && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" && parsed.RawQuery == "" && parsed.Path == "/auth/external/callback" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func (integration *Integration[P]) approveAuthorization(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Approval validates the request before issuing a code.
	if err := httpguard.DecodeForm(writer, request, 16<<10, "request", "decision"); err != nil {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	requestID, requestOK := oneValue(request.PostForm, "request", 128)
	decision, decisionOK := oneValue(request.PostForm, "decision", 8)
	profile := integration.config.CurrentProfile(request)
	if !requestOK || !decisionOK || decision != "allow" && decision != "deny" || !profile.Owner {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	integration.mu.Lock()
	transaction, found := integration.requests[secretHash(requestID)]
	if found && transaction.Expires.After(integration.now()) && profile.ID == transaction.ProfileID {
		delete(integration.requests, secretHash(requestID))
	} else {
		found = false
	}
	integration.mu.Unlock()
	if !found {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	if decision == "deny" {
		redirectAuthorization(writer, request, transaction, url.Values{"error": {"access_denied"}})
		return
	}
	code := integration.text()
	transaction.Expires = integration.now().Add(codeTTL)
	integration.mu.Lock()
	integration.pruneOAuthLocked(integration.now())
	if len(integration.codes) >= oauthLimit {
		integration.mu.Unlock()
		oauthError(writer, "temporarily_unavailable", http.StatusTooManyRequests)
		return
	}
	integration.codes[secretHash(code)] = transaction
	integration.mu.Unlock()
	redirectAuthorization(writer, request, transaction, url.Values{"code": {code}})
}

func redirectAuthorization(writer http.ResponseWriter, request *http.Request, transaction authorization, values url.Values) {
	redirect, _ := url.Parse(transaction.RedirectURI)
	query := redirect.Query()
	for key, value := range values {
		query[key] = value
	}
	query.Set("state", transaction.State)
	redirect.RawQuery = query.Encode()
	http.Redirect(writer, request, redirect.String(), http.StatusFound) //nolint:gosec // The callback was validated before storage.
}

func (integration *Integration[P]) tokenHTTP(writer http.ResponseWriter, request *http.Request) {
	input, oauthCode := decodeTokenRequest(writer, request)
	if oauthCode != "" {
		oauthError(writer, oauthCode, http.StatusBadRequest)
		return
	}
	transaction, found := integration.takeCode(input)
	if !found {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	profile, found := integration.config.FindProfile(transaction.ProfileID)
	if !found || !profile.Owner {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	token, err := integration.config.CreateKey(profile.Source, "Home Assistant")
	if err != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	server := integration.config.Server()
	writeJSON(writer, map[string]any{"access_token": token, "token_type": "Bearer", "serverId": server.ID, "name": server.Name}, http.StatusOK)
}

func decodeTokenRequest(writer http.ResponseWriter, request *http.Request) (tokenRequest, string) { //nolint:cyclop // Token input validation is explicit and fail-closed.
	if !formEncoded(request) || request.Header.Get("Authorization") != "" || request.URL.RawQuery != "" || request.ContentLength > 16<<10 {
		return tokenRequest{}, "invalid_request"
	}
	if err := httpguard.DecodeForm(writer, request, 16<<10, "grant_type", "client_id", "code", "redirect_uri", "code_verifier"); err != nil {
		return tokenRequest{}, "invalid_request"
	}
	grant, grantOK := oneValue(request.PostForm, "grant_type", 32)
	requestedClient, clientOK := oneValue(request.PostForm, "client_id", 32)
	code, codeOK := oneValue(request.PostForm, "code", 128)
	redirectURI, redirectOK := oneValue(request.PostForm, "redirect_uri", 2048)
	verifier, verifierOK := oneValue(request.PostForm, "code_verifier", 128)
	if !grantOK || !clientOK || !codeOK || !redirectOK || !verifierOK || grant != "authorization_code" || requestedClient != clientID || !validPKCEValue(verifier) || !validRedirect(redirectURI) {
		return tokenRequest{}, "invalid_grant"
	}
	return tokenRequest{code, redirectURI, verifier}, ""
}

func (integration *Integration[P]) takeCode(input tokenRequest) (authorization, bool) {
	digest := sha256.Sum256([]byte(input.Verifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	integration.mu.Lock()
	defer integration.mu.Unlock()
	transaction, found := integration.codes[secretHash(input.Code)]
	if found && transaction.Expires.After(integration.now()) && transaction.RedirectURI == input.RedirectURI && subtle.ConstantTimeCompare([]byte(wantChallenge), []byte(transaction.Challenge)) == 1 {
		delete(integration.codes, secretHash(input.Code))
	} else {
		found = false
	}
	return transaction, found
}

func (integration *Integration[P]) pruneOAuthLocked(now time.Time) {
	for id, request := range integration.requests {
		if !request.Expires.After(now) {
			delete(integration.requests, id)
		}
	}
	for id, code := range integration.codes {
		if !code.Expires.After(now) {
			delete(integration.codes, id)
		}
	}
}

func oneValue(values url.Values, key string, limit int) (string, bool) {
	value := ""
	if len(values[key]) == 1 {
		value = values[key][0]
	}
	return value, len(values[key]) == 1 && value != "" && len(value) <= limit
}

func onlyValues(values url.Values, allowed ...string) bool {
	for key := range values {
		if !slices.Contains(allowed, key) {
			return false
		}
	}
	return true
}

func formEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil
}

func secretHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func oauthError(writer http.ResponseWriter, code string, status int) {
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]string{"error": code}, status)
}
