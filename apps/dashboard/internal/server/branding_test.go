package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestBrandUsesAsymmetricWindowsLogo(t *testing.T) {
	app := newTestApplication(t, Config{})
	icon := app.request(t, http.MethodGet, "/static/icon.svg", "", nil)
	setup := app.request(t, http.MethodGet, "/setup", "", nil)

	if icon.Code != http.StatusOK || setup.Code != http.StatusOK {
		t.Fatalf("icon status = %d, setup status = %d", icon.Code, setup.Code)
	}
	for _, expected := range []string{`d="M128 128h112v256H128Z"`, `d="M272 128h112v112H272Z"`, `d="M272 272h112v112H272Z"`} {
		if !strings.Contains(icon.Body.String(), expected) {
			t.Fatalf("icon does not contain window geometry %q: %s", expected, icon.Body.String())
		}
	}
	if !strings.Contains(setup.Body.String(), `/static/icon.svg?v=5`) {
		t.Fatalf("setup does not use the current logo revision: %s", setup.Body.String())
	}
}
