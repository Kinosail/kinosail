package server

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

type NotificationConfig = auditjournal.NotificationConfig

type notificationAdapter struct {
	shared *auditjournal.Notification
	status string
}

func newNotification(config NotificationConfig) *notificationAdapter {
	shared := auditjournal.NewNotification(config, localIntegrationHTTPClient(10*time.Second))
	return &notificationAdapter{shared: shared, status: shared.Status()}
}

func (adapter *notificationAdapter) send(ctx context.Context, event auditEvent) {
	adapter.shared.Send(ctx, event)
}
