package servertest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/mediashares"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
)

// PublicAuthorizationArtifacts captures the public artifacts after the real app's kill switch runs.
type PublicAuthorizationArtifacts struct {
	Sessions                  int
	QuickPresent              bool
	Passkeys                  *sharedpasskeys.Engine
	PublicCookie, LocalCookie *http.Cookie
	Shares                    *mediashares.Store
}

// AssertPublicAuthorizationRevoked checks revocation while retaining local passkey ceremonies.
func AssertPublicAuthorizationRevoked(t *testing.T, artifacts PublicAuthorizationArtifacts) {
	t.Helper()
	if artifacts.Sessions != 0 || artifacts.QuickPresent {
		t.Fatalf("public authorization survived: sessions=%d quick=%v", artifacts.Sessions, artifacts.QuickPresent)
	}
	publicRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/finish", strings.NewReader(`{}`))
	publicRequest.AddCookie(artifacts.PublicCookie)
	if _, _, finishErr := artifacts.Passkeys.FinishLoginRequest(publicRequest, nil); !errors.Is(finishErr, sharedpasskeys.ErrCeremonyExpired) {
		t.Fatalf("public passkey ceremony survived: %v", finishErr)
	}
	localRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/finish", strings.NewReader(`{}`))
	localRequest.AddCookie(artifacts.LocalCookie)
	if _, _, finishErr := artifacts.Passkeys.FinishLoginRequest(localRequest, nil); errors.Is(finishErr, sharedpasskeys.ErrCeremonyExpired) {
		t.Fatal("local passkey ceremony was revoked")
	}
	if active, listErr := artifacts.Shares.List(); listErr != nil || len(active) != 0 {
		t.Fatalf("Media Shares survived: shares=%d error=%v", len(active), listErr)
	}
}
