package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type transcoderCheckResult = transcodepolicy.CheckResult

func (store *settingsStore) runTranscoderCheck(ctx context.Context) transcoderCheckResult {
	options, _ := store.transcoderState().CheckSettings(store.hardware)
	result := transcodepolicy.Check(ctx, store.ffmpeg, options, store.hardware.Backend(options.Accelerator).Name)
	store.hardware.RecordCheck(options, result)
	return store.recordTranscoderCheck(result)
}

func (store *settingsStore) recordTranscoderCheck(result transcoderCheckResult) transcoderCheckResult {
	store.mu.Lock()
	store.transcoderCheck = result
	store.mu.Unlock()
	return result
}

func (store *settingsStore) currentTranscoderCheck() transcoderCheckResult {
	store.mu.RLock()
	result := store.transcoderCheck
	store.mu.RUnlock()
	if result.Status == "" {
		options := store.transcoding()
		result = transcodepolicy.PendingCheck(options, store.hardware.Backend(options.Accelerator).Name)
	}
	return result
}

func testTranscoder(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		settings.runTranscoderCheck(request.Context())
		http.Redirect(writer, request, "/settings#transcoder", http.StatusSeeOther)
	}
}
