package auditjournal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

type NotificationConfig struct{ URL, Token string }

type Notification struct {
	config NotificationConfig
	client *http.Client
	status string
}

func NewNotification(config NotificationConfig, client *http.Client) *Notification {
	notification := &Notification{config: config, client: client, status: "Not configured"}
	if config.URL == "" {
		return notification
	}
	target, err := url.Parse(config.URL)
	if err != nil {
		return invalidNotification(notification)
	}
	if client == nil {
		return invalidNotification(notification)
	}
	if target.Host == "" {
		return invalidNotification(notification)
	}
	if target.User != nil {
		return invalidNotification(notification)
	}
	if target.Fragment != "" {
		return invalidNotification(notification)
	}
	if target.Scheme != "https" {
		if !loopbackHTTP(target) {
			return invalidNotification(notification)
		}
	}
	notification.status = "Enabled"
	return notification
}

func invalidNotification(notification *Notification) *Notification {
	notification.status = "Configuration error"
	slog.Warn("invalid notification webhook configuration")
	return notification
}

func (notification *Notification) Status() string {
	if notification == nil {
		return "Configuration error"
	}
	return notification.status
}

func (notification *Notification) Send(ctx context.Context, event Event) {
	if notification == nil || notification.status != "Enabled" {
		return
	}
	payload, _ := json.Marshal(event)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, notification.config.URL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	if notification.config.Token != "" {
		request.Header.Set("Authorization", "Bearer "+notification.config.Token)
	}
	response, err := notification.client.Do(request) //nolint:gosec // The installation-owned URL is validated above.
	if err != nil {
		slog.Warn("notification webhook failed", "error", err)
		return
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if !webhookStatusAccepted(response.StatusCode) {
		slog.Warn("notification webhook rejected event", "status", response.StatusCode)
	}
}

func webhookStatusAccepted(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func loopbackHTTP(target *url.URL) bool {
	if target.Scheme != "http" {
		return false
	}
	host := target.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
