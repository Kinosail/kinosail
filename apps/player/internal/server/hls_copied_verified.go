package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const (
	copiedHLSSettledAge         = 2 * time.Second
	copiedHLSVerificationMaxAge = 10 * time.Minute
	copiedHLSAdmissionSlots     = 2
)

var errCopiedHLSBusy = errors.New("copied HLS timeline verification is busy")

type copiedHLSVerifiedAssets struct {
	initialization, first [32]byte
	initInfo, firstInfo   os.FileInfo
	verified              time.Time
}

type copiedHLSRetryKey struct{}

type copiedHLSRetry struct{ busy bool }

func copiedHLSRenditionIdentity(rendition *os.Root) (os.FileInfo, os.FileInfo, bool) {
	initialization, err := rendition.Lstat("init.mp4")
	if err != nil || !initialization.Mode().IsRegular() {
		return nil, nil, false
	}
	first, err := rendition.Lstat("segment-00000.m4s")
	if os.IsNotExist(err) {
		return initialization, nil, true
	}
	if err != nil || !first.Mode().IsRegular() {
		return nil, nil, false
	}
	return initialization, first, true
}

func sameOptionalCopiedHLSFile(first, second os.FileInfo) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return sameCopiedHLSFile(first, second)
}

func (manager *hlsManager) copiedHLSAssetsVerified(key string, certificate copiedHLSClockCertificate, initialization, first os.FileInfo) bool {
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	value, ok := metadata.verified[key]
	metadata.mu.Unlock()
	return ok && time.Since(value.verified) < copiedHLSVerificationMaxAge && value.initialization == certificate.Initialization &&
		(first == nil || value.first == certificate.First) && sameCopiedHLSFile(value.initInfo, initialization) && sameOptionalCopiedHLSFile(value.firstInfo, first)
}

func (manager *hlsManager) rememberCopiedHLSAssets(key string, certificate copiedHLSClockCertificate, initialization, first os.FileInfo) {
	settled := time.Now().Add(-copiedHLSSettledAge)
	if initialization.ModTime().After(settled) || first != nil && first.ModTime().After(settled) {
		return
	}
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	if metadata.verified == nil || len(metadata.verified) >= 64 {
		metadata.verified = make(map[string]copiedHLSVerifiedAssets)
	}
	metadata.verified[key] = copiedHLSVerifiedAssets{initialization: certificate.Initialization, first: certificate.First, initInfo: initialization, firstInfo: first, verified: time.Now()}
	metadata.mu.Unlock()
}

func (manager *hlsManager) verifyCopiedHLSRenditionAssetsOnce(ctx context.Context, key string, rendition *os.Root, certificate copiedHLSClockCertificate) error {
	initialization, first, ok := copiedHLSRenditionIdentity(rendition)
	if !ok {
		return errCopiedHLSIndex
	}
	if manager.copiedHLSAssetsVerified(key, certificate, initialization, first) {
		return nil
	}
	ctx, release, err := manager.copiedHLSClockAdmission(ctx)
	if err != nil {
		return err
	}
	defer release()
	if manager.copiedHLSAssetsVerified(key, certificate, initialization, first) {
		return nil
	}
	if err := verifyCopiedHLSRenditionAssets(ctx, rendition, certificate); err != nil {
		return err
	}
	manager.rememberCopiedHLSAssets(key, certificate, initialization, first)
	return nil
}

func withCopiedHLSRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, copiedHLSRetryKey{}, &copiedHLSRetry{})
}

func markCopiedHLSBusy(ctx context.Context, mode string) {
	if retry, ok := ctx.Value(copiedHLSRetryKey{}).(*copiedHLSRetry); ok {
		retry.busy = true
	}
	slog.WarnContext(ctx, "HLS copied timeline verification busy", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", mode, "failure_class", "verification-busy")
}

func copiedHLSRetryable(ctx context.Context) bool {
	retry, ok := ctx.Value(copiedHLSRetryKey{}).(*copiedHLSRetry)
	return ok && retry.busy
}

func hlsRetryLater(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Retry-After", "2")
	localizedError(writer, request, "compatible playback is unavailable", http.StatusServiceUnavailable)
}
