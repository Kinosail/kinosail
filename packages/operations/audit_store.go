package operations

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

type AuditStore struct {
	*auditjournal.HTTPTracker
	RequestTelemetry
}

func NewAuditStore(ctx context.Context, dataDir string, notify func(auditjournal.Event), requests auditjournal.HTTPConfig, retentions ...time.Duration) *AuditStore {
	config := auditjournal.Config{DataDir: dataDir, Notify: notify}
	if len(retentions) > 0 {
		config.AuditRetention = retentions[0]
	}
	if len(retentions) > 1 {
		config.PlaybackRetention = retentions[1]
	}
	return &AuditStore{HTTPTracker: auditjournal.NewHTTPTracker(auditjournal.New(ctx, config), requests)}
}
