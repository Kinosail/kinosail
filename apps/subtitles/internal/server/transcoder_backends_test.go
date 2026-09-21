package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestEveryHardwareBackendHasAnFFmpegRecipe(t *testing.T) {
	servertest.EveryHardwareBackendHasAnFFmpegRecipe(t, videoArguments)
}
func TestRKMPPToneMappingUsesSoftwareDecodeAndFilterBeforeHardwareEncode(t *testing.T) {
	servertest.RKMPPToneMappingUsesSoftwareDecodeAndFilterBeforeHardwareEncode(t, videoArguments)
}
func TestIntelBackendsUseTheDetectedRenderDevice(t *testing.T) {
	servertest.IntelBackendsUseTheDetectedRenderDevice(t, videoArguments)
}
