package server

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/operations"
	"github.com/go-webauthn/webauthn/webauthn"
)

type (
	auditEvent  = auditjournal.Event
	auditWriter = auditjournal.ResponseWriter
)

type auditStore struct {
	*auditjournal.HTTPTracker
	requests      atomic.Uint64
	requestErrors atomic.Uint64
	panics        atomic.Uint64
	failures      operations.FailureLog
}

func newAuditStore(ctx context.Context, dataDir string, notify func(auditEvent), retentions ...time.Duration) *auditStore {
	var auditRetention, playbackRetention time.Duration
	if len(retentions) > 0 {
		auditRetention = retentions[0]
	}
	if len(retentions) > 1 {
		playbackRetention = retentions[1]
	}
	journal := auditjournal.New(ctx, auditjournal.Config{DataDir: dataDir, AuditRetention: auditRetention, PlaybackRetention: playbackRetention, Notify: notify})
	return &auditStore{HTTPTracker: auditjournal.NewHTTPTracker(journal, auditjournal.HTTPConfig{
		Action: auditAction, Details: auditDetails, Actor: auditActor, FallbackActor: auditFallbackActor,
		Target: auditTarget, MarkSecurity: markSecurityRecorded, RequestID: activityRequestID,
		Remote: remoteIP, NewID: randID,
	})}
}

func auditActor(request *http.Request) auditjournal.Actor {
	profile := currentViewer(request)
	return auditjournal.Actor{Name: profile.Name, ID: profile.ID}
}

func auditFallbackActor(request *http.Request) auditjournal.Actor {
	profile := auditViewer(request)
	return auditjournal.Actor{Name: profile.Name, ID: profile.ID}
}

func (store *auditStore) passkeyRisk(request *http.Request, profile viewerProfile, credential *webauthn.Credential) {
	store.PasskeyRisk(request, auditjournal.Actor{Name: profile.Name, ID: profile.ID}, credential.ID, credential.Authenticator.SignCount, credential.Flags.BackupEligible, credential.Flags.BackupState)
}
