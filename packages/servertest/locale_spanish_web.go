package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// WebSurfacesCanUseSpanish preserves localized chrome, account controls and cookie-selected settings.
func (fixture LocaleWebFixture) WebSurfacesCanUseSpanish(t *testing.T, signIn func(*testing.T, http.Handler, string, string) *http.Cookie, web AuthCookieRequest, settingsMarker string) {
	t.Parallel()
	handler := fixture.NewHandler(t, false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?lang=es", nil))
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), `<html lang="es" dir="ltr"`) || !strings.Contains(home.Body.String(), ">Inicio<") {
		t.Fatalf("Spanish home = %d %q", home.Code, home.Body.String())
	}
	MustContainAll(t, home.Body.String(), ">Audiolibros<", ">Historial<")
	MustContainAll(t, home.Body.String(), `aria-label="Acciones"`, `data-command-label="Películas">Películas<`, `data-command-label="Descargas sin conexión">Descargas sin conexión<`, `aria-label="Cancelar"`)
	authenticated := fixture.NewHandler(t, true)
	cookie := signIn(t, authenticated, "/setup", "name=Owner&password=owner-password")
	account := web(t, authenticated, http.MethodGet, "/?lang=es", "", cookie)
	MustContainAll(t, account.Body.String(), `aria-label="Perfil"`, "Cerrar sesión")
	cookies := home.Result().Cookies()
	settings := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	request.AddCookie(cookies[0])
	handler.ServeHTTP(settings, request)
	if settings.Code != http.StatusOK || strings.Contains(settings.Body.String(), "OpenSubtítulos") || strings.Contains(settings.Body.String(), "Protected automatically") || strings.Contains(settings.Body.String(), "Browse library") {
		t.Fatalf("Spanish settings = %d %q", settings.Code, settings.Body.String())
	}
	MustContainAll(t, settings.Body.String(), "Configuración del servidor", settingsMarker, "Automática (Recomendado)", "Guardar programación", "Protección automática", "almacenamiento cifrado")
	for path, expected := range map[string][]string{
		"/settings/backups": {"Copias de seguridad automáticas", "Estado", "Destino", "Acciones", "Recuperación ante desastres"},
	} {
		response := httptest.NewRecorder()
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		request.AddCookie(cookies[0])
		handler.ServeHTTP(response, request)
		MustContainAll(t, response.Body.String(), expected...)
	}
}

// SetupAndLoginCanUseSpanish retains the original Spanish setup and login assertions.
func (fixture LocaleWebFixture) SetupAndLoginCanUseSpanish(t *testing.T) {
	t.Parallel()
	handler := fixture.NewHandler(t, false)
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup?lang=es", nil))
	if !strings.Contains(setup.Body.String(), "Propietario account") || !strings.Contains(setup.Body.String(), `lang="es"`) {
		t.Fatalf("Spanish setup = %q", setup.Body.String())
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?lang=es", nil))
	if !strings.Contains(login.Body.String(), "Inicia sesión en tu universo multimedia privado.") || !strings.Contains(login.Body.String(), "Iniciar sesión con clave de acceso") {
		t.Fatalf("Spanish login = %q", login.Body.String())
	}
}
