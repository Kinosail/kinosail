package mcpgateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func (adapter *Gateway) verifyEventCallback(ctx context.Context, subscription eventSubscription) error { //nolint:cyclop // Challenge, status, and cache validation form one callback verification.
	key := subscription.Scope + "\x00" + subscription.URL
	adapter.eventMu.Lock()
	verified := adapter.eventVerified[key] > adapter.connections.now().Unix()
	adapter.eventMu.Unlock()
	if verified {
		return nil
	}
	parsed, _ := url.Parse(subscription.URL)
	if !adapter.eventVerify.Allow(parsed.Hostname(), 10) {
		return eventError(-32013, "callback verification limit reached", map[string]string{"limit": "verification"})
	}
	challenge := rand.Text()
	body, _ := json.Marshal(map[string]string{"type": "verification", "challenge": challenge})
	response, err := adapter.sendEventWebhook(ctx, subscription, "msg_verification_"+rand.Text(), body)
	if err != nil {
		return eventError(-32015, "callback verification failed", map[string]string{"reason": eventCallbackError(err)})
	}
	defer response.Body.Close()
	var echoed struct {
		Challenge string `json:"challenge"`
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return eventError(-32015, "callback verification failed", map[string]string{"reason": eventResponseError(response.StatusCode)})
	}
	if httpguard.DecodeUniqueJSON(response.Body, 4096, &echoed) != nil || subtle.ConstantTimeCompare([]byte(challenge), []byte(echoed.Challenge)) != 1 {
		return eventError(-32015, "callback verification failed", map[string]string{"reason": "challenge_failed"})
	}
	adapter.eventMu.Lock()
	for candidate, until := range adapter.eventVerified {
		if until <= adapter.connections.now().Unix() {
			delete(adapter.eventVerified, candidate)
		}
	}
	if len(adapter.eventVerified) < eventSubscriptionLimit {
		adapter.eventVerified[key] = adapter.connections.now().Add(5 * time.Minute).Unix()
	}
	adapter.eventMu.Unlock()
	return nil
}

func (adapter *Gateway) sendEventWebhook(ctx context.Context, subscription eventSubscription, id string, body []byte) (*http.Response, error) {
	if len(body) > 256<<10 {
		return nil, errors.New("event payload exceeds limit")
	}
	callback, err := eventURL(subscription.URL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, callback, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(adapter.connections.now().Unix(), 10)
	signature, err := eventSignature(subscription.Secret, id, timestamp, body)
	if err != nil {
		return nil, err
	}
	if subscription.OldSecret != "" && subscription.RotateUntil > adapter.connections.now().Unix() {
		old, err := eventSignature(subscription.OldSecret, id, timestamp, body)
		if err != nil {
			return nil, err
		}
		signature += " " + old
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("webhook-id", id)
	request.Header.Set("webhook-timestamp", timestamp)
	request.Header.Set("webhook-signature", signature)
	request.Header.Set("X-MCP-Subscription-Id", subscription.ID)
	return adapter.connections.client.Do(request)
}

func eventSignature(secret, id, timestamp string, body []byte) (string, error) {
	key, err := eventSecret(secret)
	if err != nil {
		return "", err
	}
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write([]byte(id + "." + timestamp + "."))
	_, _ = signer.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(signer.Sum(nil)), nil
}

func eventCallbackError(err error) string {
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "timeout"
	}
	if strings.Contains(err.Error(), "tls") || strings.Contains(err.Error(), "certificate") {
		return "tls_error"
	}
	return "connection_refused"
}

func eventResponseError(status int) string {
	if status >= 500 {
		return "http_5xx"
	}
	return "http_4xx"
}
