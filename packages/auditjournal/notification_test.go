package auditjournal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestNotificationValidatesConfigurationBeforeDelivery(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	for name, test := range map[string]struct {
		url    string
		client *http.Client
		want   string
	}{
		"empty":           {client: client, want: "Not configured"},
		"parse":           {url: "://", client: client, want: "Configuration error"},
		"missing host":    {url: "https:/path", client: client, want: "Configuration error"},
		"user":            {url: "https://user@example.com", client: client, want: "Configuration error"},
		"fragment":        {url: "https://example.com/#fragment", client: client, want: "Configuration error"},
		"plain external":  {url: "http://example.com", client: client, want: "Configuration error"},
		"plain public IP": {url: "http://192.0.2.1", client: client, want: "Configuration error"},
		"nil client":      {url: "https://example.com", want: "Configuration error"},
		"https":           {url: "https://example.com", client: client, want: "Enabled"},
		"localhost":       {url: "http://localhost/hook", client: client, want: "Enabled"},
		"ipv4 loopback":   {url: "http://127.0.0.1/hook", client: client, want: "Enabled"},
		"ipv6 loopback":   {url: "http://[::1]/hook", client: client, want: "Enabled"},
	} {
		if got := NewNotification(NotificationConfig{URL: test.url}, test.client).Status(); got != test.want {
			t.Fatalf("%s status = %q, want %q", name, got, test.want)
		}
	}
	var notification *Notification
	if notification.Status() != "Configuration error" {
		t.Fatal("nil notification status was enabled")
	}
	if loopbackHTTP(&url.URL{Scheme: "https", Host: "localhost"}) {
		t.Fatal("HTTPS was classified as loopback HTTP")
	}
	for status, accepted := range map[int]bool{http.StatusEarlyHints: false, http.StatusOK: true, 299: true, http.StatusMultipleChoices: false} {
		if webhookStatusAccepted(status) != accepted {
			t.Fatalf("webhook status %d accepted = %v", status, webhookStatusAccepted(status))
		}
	}
}

func TestNotificationSendsBoundedAuthenticatedJSON(t *testing.T) {
	t.Parallel()
	received := make(chan *http.Request, 1)
	body := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, _ := io.ReadAll(request.Body)
		received <- request
		body <- string(data)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	notification := NewNotification(NotificationConfig{URL: server.URL, Token: "secret"}, server.Client())
	if notification.Status() != "Enabled" {
		t.Fatalf("local notification status = %q", notification.Status())
	}
	notification.Send(t.Context(), Event{ID: "event", Time: "2026-09-04T12:00:00Z", Category: "security", Action: "access.denied", Result: "denied"})
	request := notificationValue(t, received)
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Authorization") != "Bearer secret" || !strings.Contains(notificationValue(t, body), `"action":"access.denied"`) {
		t.Fatalf("request = %#v", request)
	}
	withoutToken := NewNotification(NotificationConfig{URL: server.URL}, server.Client())
	if withoutToken.Status() != "Enabled" {
		t.Fatalf("tokenless notification status = %q", withoutToken.Status())
	}
	withoutToken.Send(t.Context(), Event{ID: "second", Time: "2026-09-04T12:00:00Z", Category: "security", Action: "test", Result: "success"})
	if request := notificationValue(t, received); request.Header.Get("Authorization") != "" {
		t.Fatalf("unexpected authorization = %q", request.Header.Get("Authorization"))
	}
	notificationValue(t, body)
}

func notificationValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatal("notification was not delivered")
		var zero T
		return zero
	}
}

func TestNotificationHandlesDisabledRequestTransportAndResponseFailures(t *testing.T) {
	t.Parallel()
	called := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called++
		return nil, errors.New("transport failed")
	})}
	NewNotification(NotificationConfig{}, client).Send(t.Context(), Event{})
	var nilNotification *Notification
	nilNotification.Send(t.Context(), Event{})
	failed := NewNotification(NotificationConfig{URL: "https://example.com"}, client)
	failed.Send(t.Context(), Event{})
	if called != 1 {
		t.Fatalf("transport calls = %d", called)
	}
	badRequest := &Notification{config: NotificationConfig{URL: "://"}, client: client, status: "Enabled"}
	badRequest.Send(t.Context(), Event{})
	if called != 1 {
		t.Fatal("invalid request reached transport")
	}
	rejected := &Notification{
		config: NotificationConfig{URL: "https://example.com"}, status: "Enabled",
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 8192)))}, nil
		})},
	}
	rejected.Send(context.Background(), Event{})
}
