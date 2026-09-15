package identitycore

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxBody:             4,
		Secure:              func(*http.Request) bool { return true },
		UnsafeCrossOrigin:   func(*http.Request) bool { return false },
		SessionCSRFRequired: func(*http.Request) bool { return false },
		ValidCSRF:           func(*http.Request) bool { return true },
		Reject: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			http.Error(writer, message, status)
		},
	}
}

func TestSecurityRejectsInvalidConfigWithoutCallingNext(t *testing.T) {
	t.Parallel()
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	valid := validSecurityConfig()
	configs := []SecurityConfig{{}}
	mutations := []func(*SecurityConfig){
		func(config *SecurityConfig) { config.MaxBody = 0 },
		func(config *SecurityConfig) { config.Secure = nil },
		func(config *SecurityConfig) { config.UnsafeCrossOrigin = nil },
		func(config *SecurityConfig) { config.SessionCSRFRequired = nil },
		func(config *SecurityConfig) { config.ValidCSRF = nil },
		func(config *SecurityConfig) { config.Reject = nil },
	}
	for _, mutate := range mutations {
		config := valid
		mutate(&config)
		configs = append(configs, config)
	}
	for _, config := range configs {
		response := httptest.NewRecorder()
		Security(next, config).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if response.Code != http.StatusInternalServerError || called {
			t.Fatalf("invalid config response=%d called=%v", response.Code, called)
		}
	}
	response := httptest.NewRecorder()
	Security(nil, valid).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("nil next response = %d", response.Code)
	}
}

func TestSecurityAppliesHeadersAndCallsNext(t *testing.T) {
	t.Parallel()
	called := false
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		body, err := io.ReadAll(request.Body)
		if err != nil || string(body) != "data" {
			t.Fatalf("body = %q, %v", body, err)
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("data"))
	Security(next, validSecurityConfig()).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("safe request response=%d called=%v", response.Code, called)
	}
	for _, header := range []string{"Cache-Control", "Content-Security-Policy", "Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy", "Permissions-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options", "X-XSS-Protection", "Strict-Transport-Security"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing security header %q", header)
		}
	}
	config := validSecurityConfig()
	config.Secure = func(*http.Request) bool { return false }
	response = httptest.NewRecorder()
	Security(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), config).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("insecure request received HSTS")
	}
}

func TestSecurityRejectsBeforeNextSideEffect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*SecurityConfig)
		body   string
		want   int
	}{
		{"oversized", func(*SecurityConfig) {}, "large", http.StatusRequestEntityTooLarge},
		{"cross origin", func(config *SecurityConfig) { config.UnsafeCrossOrigin = func(*http.Request) bool { return true } }, "data", http.StatusForbidden},
		{"invalid CSRF", func(config *SecurityConfig) {
			config.SessionCSRFRequired = func(*http.Request) bool { return true }
			config.ValidCSRF = func(*http.Request) bool { return false }
		}, "data", http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			config := validSecurityConfig()
			test.mutate(&config)
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(test.body))
			Security(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), config).ServeHTTP(response, request)
			if response.Code != test.want || called {
				t.Fatalf("rejection response=%d called=%v", response.Code, called)
			}
		})
	}
}

func TestSecuritySeparatesOriginAndCSRFPredicates(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*SecurityConfig){
		"valid required CSRF": func(config *SecurityConfig) {
			config.SessionCSRFRequired = func(*http.Request) bool { return true }
			config.ValidCSRF = func(*http.Request) bool { return true }
		},
		"irrelevant invalid CSRF": func(config *SecurityConfig) {
			config.ValidCSRF = func(*http.Request) bool { return false }
		},
	} {
		config := validSecurityConfig()
		mutate(&config)
		called := false
		response := httptest.NewRecorder()
		Security(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), config).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
		if !called || response.Code != http.StatusOK {
			t.Fatalf("%s response=%d called=%v", name, response.Code, called)
		}
	}
	config := validSecurityConfig()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Body = nil
	called := false
	Security(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), config).ServeHTTP(httptest.NewRecorder(), request)
	if !called {
		t.Fatal("nil request body did not reach the application")
	}
}

func TestSecurityBoundsUnknownLengthBodiesAndLeavesSafeMethodsReadable(t *testing.T) {
	t.Parallel()
	config := validSecurityConfig()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Body = io.NopCloser(strings.NewReader("large"))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	Security(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := io.ReadAll(request.Body); err == nil {
			t.Fatal("unknown-length oversized body remained unbounded")
		}
	}), config).ServeHTTP(response, request)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request = httptest.NewRequestWithContext(t.Context(), method, "/", strings.NewReader("large"))
		response = httptest.NewRecorder()
		called := false
		Security(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			called = true
			body, err := io.ReadAll(request.Body)
			if err != nil || string(body) != "large" {
				t.Fatalf("%s body = %q, %v", method, body, err)
			}
		}), config).ServeHTTP(response, request)
		if !called || response.Code != http.StatusOK {
			t.Fatalf("%s response=%d called=%v", method, response.Code, called)
		}
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Body = nil
	request.ContentLength = config.MaxBody + 1
	response = httptest.NewRecorder()
	called := false
	Security(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), config).ServeHTTP(response, request)
	if !called || response.Code != http.StatusOK {
		t.Fatalf("nil body response=%d called=%v", response.Code, called)
	}
}
