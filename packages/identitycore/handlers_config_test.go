package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDenialConfigurationRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	valid := (&recordedResponse{}).denialConfig(false)
	mutations := []func(*DenialConfig){
		func(config *DenialConfig) { config.Denied = nil },
		func(config *DenialConfig) { config.NotFound = nil },
		func(config *DenialConfig) { config.AuthenticationError = nil },
		func(config *DenialConfig) { config.Error = nil },
		func(config *DenialConfig) { config.JSON = nil },
	}
	for index, mutate := range mutations {
		config := valid
		mutate(&config)
		response := httptest.NewRecorder()
		RespondDenial(response, request, OwnerRequired, config)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("invalid denial dependency %d response = %d", index, response.Code)
		}
	}
}

func TestOwnerConfigurationAndRecentAuthenticationBoundary(t *testing.T) {
	t.Parallel()
	record := &recordedResponse{}
	valid := ownerConfig(record, true, false, false, true, false)
	mutations := []func(*OwnerConfig){
		func(config *OwnerConfig) { config.Identity = nil },
		func(config *OwnerConfig) { config.Managed = nil },
		func(config *OwnerConfig) { config.RecentlyAuthenticated = nil },
		func(config *OwnerConfig) { config.AuthenticationError = nil },
		func(config *OwnerConfig) { config.StepUpPath = nil },
		func(config *OwnerConfig) { config.Error = nil },
		func(config *OwnerConfig) { config.JSON = nil },
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	for index, mutate := range mutations {
		config := valid
		mutate(&config)
		response := httptest.NewRecorder()
		Owner(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid config called next") }), config).ServeHTTP(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("invalid Owner dependency %d response = %d", index, response.Code)
		}
	}
	for _, exemption := range []struct {
		local, managed bool
		method         string
	}{{true, false, http.MethodPost}, {false, true, http.MethodPost}, {false, false, http.MethodGet}, {false, false, http.MethodHead}} {
		config := ownerConfig(record, true, exemption.local, exemption.managed, false, false)
		config.RecentlyAuthenticated = func(*http.Request, time.Duration) bool {
			t.Fatal("recent authentication was evaluated for an exempt Owner")
			return false
		}
		Owner(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), config).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), exemption.method, "/", nil))
	}
	called := 0
	config := ownerConfig(record, true, false, false, false, false)
	config.RecentlyAuthenticated = func(_ *http.Request, window time.Duration) bool {
		called++
		if window != 10*time.Minute {
			t.Fatalf("recent authentication window = %s", window)
		}
		return true
	}
	Owner(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), config).ServeHTTP(httptest.NewRecorder(), request)
	if called != 1 {
		t.Fatalf("recent authentication calls = %d", called)
	}
}

func TestPasswordLoginConfigurationRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
	mutations := []func(*PasswordLoginConfig[string]){
		func(config *PasswordLoginConfig[string]) { config.Authenticate = nil },
		func(config *PasswordLoginConfig[string]) { config.ProfileID = nil },
		func(config *PasswordLoginConfig[string]) { config.SignIn = nil },
		func(config *PasswordLoginConfig[string]) { config.SetAudit = nil },
		func(config *PasswordLoginConfig[string]) { config.ReturnPath = nil },
		func(config *PasswordLoginConfig[string]) { config.Error = nil },
	}
	for index, mutate := range mutations {
		record := &recordedResponse{}
		signedIn, audited := false, false
		config := passwordConfig(record, true, nil, &signedIn, &audited)
		mutate(&config)
		response := httptest.NewRecorder()
		PasswordLoginRequest(response, request, "", config)
		if response.Code != http.StatusInternalServerError || signedIn || audited {
			t.Fatalf("invalid password dependency %d response=%d signedIn=%v audited=%v", index, response.Code, signedIn, audited)
		}
	}
}

func TestMFAHTTPConfigurationRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	valid := MFAHTTPConfig{
		ReadJSON:          func(http.ResponseWriter, *http.Request, any) bool { return true },
		Setup:             func(*http.Request) (Enrollment, error) { return Enrollment{}, nil },
		Confirm:           func(*http.Request, string) error { return nil },
		MarkStrong:        func(*http.Request) error { return nil },
		Verify:            func(*http.Request, string) bool { return true },
		Disable:           func(*http.Request) error { return nil },
		Error:             func(http.ResponseWriter, error, int) {},
		JSON:              func(http.ResponseWriter, any, int) {},
		WebError:          func(http.ResponseWriter, *http.Request, error, int) {},
		WebConfirmSuccess: func(http.ResponseWriter, *http.Request) {},
		WebDisableSuccess: func(http.ResponseWriter, *http.Request) {},
	}
	mutations := []func(*MFAHTTPConfig){
		func(config *MFAHTTPConfig) { config.ReadJSON = nil },
		func(config *MFAHTTPConfig) { config.Setup = nil },
		func(config *MFAHTTPConfig) { config.Confirm = nil },
		func(config *MFAHTTPConfig) { config.MarkStrong = nil },
		func(config *MFAHTTPConfig) { config.Verify = nil },
		func(config *MFAHTTPConfig) { config.Disable = nil },
		func(config *MFAHTTPConfig) { config.Error = nil },
		func(config *MFAHTTPConfig) { config.JSON = nil },
		func(config *MFAHTTPConfig) { config.WebError = nil },
		func(config *MFAHTTPConfig) { config.WebConfirmSuccess = nil },
		func(config *MFAHTTPConfig) { config.WebDisableSuccess = nil },
	}
	for index, mutate := range mutations {
		config := valid
		mutate(&config)
		if _, err := NewMFAHandlers(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("invalid MFA HTTP dependency %d error = %v", index, err)
		}
	}
}
