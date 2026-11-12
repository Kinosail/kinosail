package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/operations"
	"github.com/go-webauthn/webauthn/webauthn"
)

type (
	auditEvent  = auditjournal.Event
	auditWriter = auditjournal.ResponseWriter
	auditStore  = operations.AuditStore
)

func newAuditStore(ctx context.Context, dataDir string, notify func(auditEvent), retentions ...time.Duration) *auditStore {
	return operations.NewAuditStore(ctx, dataDir, notify, auditjournal.HTTPConfig{
		Action: auditAction, Details: auditDetails, Actor: auditActor, FallbackActor: auditFallbackActor,
		Target: auditTarget, MarkSecurity: markSecurityRecorded, RequestID: activityRequestID,
		Remote: remoteIP, NewID: randID,
	}, retentions...)
}

func auditActor(request *http.Request) auditjournal.Actor {
	profile := currentViewer(request)
	return auditjournal.Actor{Name: profile.Name, ID: profile.ID}
}

func auditFallbackActor(request *http.Request) auditjournal.Actor {
	profile := auditViewer(request)
	return auditjournal.Actor{Name: profile.Name, ID: profile.ID}
}

func passkeyRisk(store *auditStore, request *http.Request, profile viewerProfile, credential *webauthn.Credential) {
	store.PasskeyRisk(request, auditjournal.Actor{Name: profile.Name, ID: profile.ID}, credential.ID, credential.Authenticator.SignCount, credential.Flags.BackupEligible, credential.Flags.BackupState)
}
