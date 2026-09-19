package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type transcoderCheckResult = transcodepolicy.CheckResult

func (store *settingsStore) runTranscoderCheck(ctx context.Context) transcoderCheckResult {
	return store.transcoderCheckCoordinator().Run(ctx)
}

func (store *settingsStore) currentTranscoderCheck() transcoderCheckResult {
	return store.transcoderCheckCoordinator().Current()
}

func (store *settingsStore) transcoderCheckCoordinator() transcodepolicy.CheckCoordinator {
	return transcodepolicy.NewCheckCoordinator(transcodepolicy.CheckCoordinatorDependencies{
		Lock: &store.mu, Result: &store.transcoderCheck, FFmpeg: store.ffmpeg,
		Resolve: func() (transcodepolicy.Settings, error) {
			return store.transcoderState().CheckSettings(store.hardware)
		},
		Pending:        func() transcodepolicy.Settings { return store.transcoding() },
		BackendName:    func(accelerator string) string { return store.hardware.Backend(accelerator).Name },
		RecordHardware: store.hardware.RecordCheck,
	})
}

func testTranscoder(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		settings.runTranscoderCheck(request.Context())
		http.Redirect(writer, request, "/settings#transcoder", http.StatusSeeOther)
	}
}
