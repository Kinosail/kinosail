
func TestMFASetupStepsAreTranslated(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa/setup", nil)
	request.AddCookie(cookie)
	request.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Host = "example.com"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, "Speichern Sie alle Wiederherstellungscodes, bevor Sie fortfahren.") || strings.Contains(body, "Save every recovery code before you continue.") {
		t.Fatalf("German MFA setup page = %d %q", response.Code, body)
	}
}
