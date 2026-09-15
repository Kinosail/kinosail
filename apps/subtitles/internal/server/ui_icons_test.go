package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestUIIconSetIsCompleteAndLocal(t *testing.T) {
	servertest.UIIconSetIsCompleteAndLocal(t, uiIcon)
}

func TestProductionUITemplatesAvoidMixedGlyphIcons(t *testing.T) {
	servertest.ProductionUITemplatesAvoidMixedGlyphIcons(t)
}
