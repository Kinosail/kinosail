package identitycore

import (
	"errors"
	"testing"
)

func TestPasswordLoginValidatesCallbacksBeforeAuthentication(t *testing.T) {
	t.Parallel()
	authenticated := false
	authenticate := func(string, string) (string, bool) { authenticated = true; return "owner", true }
	if _, err := passwordLogin("owner", "password", authenticate, nil); !errors.Is(err, errInvalidConfig) || authenticated {
		t.Fatalf("missing sign-in = %v, authenticated=%v", err, authenticated)
	}
	if _, err := passwordLogin[string]("owner", "password", nil, func(string) error { return nil }); !errors.Is(err, errInvalidConfig) {
		t.Fatalf("missing authentication = %v", err)
	}
}

func TestPasswordLoginRejectsCredentialsWithoutSignIn(t *testing.T) {
	t.Parallel()
	signedIn := false
	profile, err := passwordLogin("owner", "wrong", func(name, password string) (string, bool) {
		if name != "owner" || password != "wrong" {
			t.Fatalf("authentication input = %q, %q", name, password)
		}
		return "", false
	}, func(string) error { signedIn = true; return nil })
	if !errors.Is(err, errInvalidCredentials) || profile != "" || signedIn {
		t.Fatalf("invalid login = %q, %v, signedIn=%v", profile, err, signedIn)
	}
}

func TestPasswordLoginReturnsSignInFailureAndSuccess(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("session persistence failed")
	profile, err := passwordLogin("owner", "password", func(string, string) (string, bool) { return "owner-id", true }, func(string) error { return wantErr })
	if !errors.Is(err, wantErr) || profile != "" {
		t.Fatalf("failed sign-in = %q, %v", profile, err)
	}
	profile, err = passwordLogin("owner", "password", func(string, string) (string, bool) { return "owner-id", true }, func(value string) error {
		if value != "owner-id" {
			t.Fatalf("signed-in profile = %q", value)
		}
		return nil
	})
	if err != nil || profile != "owner-id" {
		t.Fatalf("successful sign-in = %q, %v", profile, err)
	}
}
