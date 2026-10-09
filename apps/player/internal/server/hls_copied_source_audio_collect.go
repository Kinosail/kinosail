package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

// Written-first operation controls retain the missing producer as an explicit
// runtime failure. This stub cannot grant readiness or a generated certificate.
func (manager *hlsManager) measureCopiedHLSSourceAudio(_ context.Context, _ library.Item, _ hlsRecipe, _ string, _ [32]byte, _, _ int64) (*copiedHLSAudioProof, error) {
	return nil, errCopiedHLSIndex
}
