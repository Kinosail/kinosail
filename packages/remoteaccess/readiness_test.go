package remoteaccess

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestSecurePublicReadinessRequiresEveryInvariant(t *testing.T) {
	t.Parallel()
	now := time.Now()
	status := Status{State: "ready", Mode: "https", Policy: "public-v1", CertificateExpires: now.Add(48 * time.Hour).UTC().Format(time.RFC3339)}
	viewer := identitycore.Profile{Remote: true, Passkeys: []webauthn.Credential{{}}}
	if readiness := SecurePublicReadiness(status, []identitycore.Profile{viewer}, nil, now); !readiness.Ready || len(readiness.Checks) != 7 {
		t.Fatalf("complete secure configuration = %#v", readiness)
	}
	for name, readiness := range map[string]Readiness{
		"owner passkey":   SecurePublicReadiness(status, []identitycore.Profile{{Owner: true, Remote: true, Passkeys: viewer.Passkeys}}, nil, now),
		"local passkey":   SecurePublicReadiness(status, []identitycore.Profile{{Passkeys: viewer.Passkeys}}, nil, now),
		"no passkey":      SecurePublicReadiness(status, []identitycore.Profile{{Remote: true}}, nil, now),
		"bad profiles":    SecurePublicReadiness(status, []identitycore.Profile{viewer}, errors.New("corrupt"), now),
		"malformed cert":  SecurePublicReadiness(Status{State: "ready", Mode: "https", Policy: "public-v1", CertificateExpires: "bad"}, []identitycore.Profile{viewer}, nil, now),
		"short cert":      SecurePublicReadiness(Status{State: "ready", Mode: "https", Policy: "public-v1", CertificateExpires: now.Add(time.Hour).UTC().Format(time.RFC3339)}, []identitycore.Profile{viewer}, nil, now),
		"wrong policy":    SecurePublicReadiness(Status{State: "ready", Mode: "https", Policy: "unknown", CertificateExpires: status.CertificateExpires}, []identitycore.Profile{viewer}, nil, now),
		"not listening":   SecurePublicReadiness(Status{State: "starting", Mode: "https", Policy: "public-v1", CertificateExpires: status.CertificateExpires}, []identitycore.Profile{viewer}, nil, now),
		"public disabled": SecurePublicReadiness(Status{State: "ready", Mode: "off", Policy: "public-v1", CertificateExpires: status.CertificateExpires}, []identitycore.Profile{viewer}, nil, now),
	} {
		if readiness.Ready {
			t.Fatalf("%s configuration was reported ready: %#v", name, readiness)
		}
	}
}

func TestReadinessViewSourceUsesBoundedBranding(t *testing.T) {
	t.Parallel()
	for product, version := range map[string]string{"Player": "75", "Subtitles": "62"} {
		source := ReadinessViewSource(product, version)
		if !strings.Contains(source, "Kinosail "+product) || !strings.Contains(source, "app.css?v="+version) || strings.Contains(source, "__PRODUCT__") {
			t.Fatalf("ReadinessViewSource(%q, %q) = %q", product, version, source)
		}
	}
	for _, values := range [][2]string{{"", "75"}, {"Player 2", "75"}, {strings.Repeat("P", 33), "75"}, {"Player", ""}, {"Player", "v75"}, {"Player", "123456789"}} {
		values := values
		t.Run(values[0]+values[1], func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid view configuration did not panic")
				}
			}()
			ReadinessViewSource(values[0], values[1])
		})
	}
}

type readinessTestView struct {
	err  error
	data Readiness
}

func (view *readinessTestView) Execute(_ http.ResponseWriter, _ *http.Request, data any) error {
	view.data = data.(Readiness)
	return view.err
}

func TestNewReadinessHandlerUsesLiveStateAndReportsViewFailure(t *testing.T) {
	t.Parallel()
	for name, manager := range map[string]*Manager{"disabled": nil, "manager": {status: Status{Mode: "https"}}} {
		view := &readinessTestView{err: errors.New("render")}
		failure := 0
		handler := NewReadinessHandler(manager, func() ([]identitycore.Profile, error) { return nil, errors.New("profiles") }, view, func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
			if message != "remote readiness view failed" || status != http.StatusInternalServerError {
				t.Fatalf("render error = %q, %d", message, status)
			}
			failure++
		})
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/remote-readiness", nil))
		if response.Header().Get("Content-Type") != "text/html; charset=utf-8" || failure != 1 || view.data.Ready {
			t.Fatalf("%s response = %#v, failures=%d, readiness=%#v", name, response.Result().Header, failure, view.data)
		}
	}
}

func TestNewReadinessHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	profiles := func() ([]identitycore.Profile, error) { return nil, nil }
	view := &readinessTestView{}
	renderError := func(http.ResponseWriter, *http.Request, string, int) {}
	for name, build := range map[string]func(){
		"profiles": func() { NewReadinessHandler(nil, nil, view, renderError) },
		"view":     func() { NewReadinessHandler(nil, profiles, nil, renderError) },
		"error":    func() { NewReadinessHandler(nil, profiles, view, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("missing dependency did not panic")
				}
			}()
			build()
		})
	}
}
