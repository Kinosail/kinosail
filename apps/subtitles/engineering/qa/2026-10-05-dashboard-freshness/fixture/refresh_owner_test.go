package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var (
	r16SetupKey = regexp.MustCompile("<code>([A-Z2-7]{32})</code>")
	r16PageCSRF = regexp.MustCompile("name=\"kinosail-csrf\" content=\"([A-Za-z0-9_-]{43})\"")
)

func r16EnrollOwner(t *testing.T, ctx context.Context, target *r16Target) error {
	t.Helper()
	password, err := r16Password()
	if err != nil {
		return err
	}
	key, err := r16SetupOwner(ctx, target, password)
	if err != nil {
		return err
	}
	csrf, err := r16OwnerToken(ctx, target, "/account")
	if err != nil {
		return err
	}
	code := servertest.TestTOTP(t, key, time.Now())
	if err = r16ConfirmMFA(ctx, target, csrf, code); err != nil {
		return err
	}
	current, err := r16OwnerToken(ctx, target, "/?view=library")
	if err != nil {
		return err
	}
	target.mu.Lock()
	target.csrf = current
	target.mu.Unlock()
	return nil
}

func r16Password() (string, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return "", errors.New("owned R16 credential generation failed")
	}
	return hex.EncodeToString(data) + "Aa7!", nil
}

func r16SecureOwnerCookie(cookies []*http.Cookie) bool {
	for _, cookie := range cookies {
		if cookie.Name == "__Host-kinosail_subtitles_session" && cookie.Secure &&
			cookie.HttpOnly && cookie.SameSite == http.SameSiteStrictMode && cookie.Path == "/" {
			return true
		}
	}
	return false
}

func r16CanonicalToken(page []byte) (string, error) {
	tokens := r16PageCSRF.FindAllSubmatch(page, -1)
	if len(tokens) != 1 || len(tokens[0]) != 2 {
		return "", errors.New("actual R16 page CSRF was not canonical")
	}
	return string(tokens[0][1]), nil
}

func r16OwnerForm(values url.Values) string {
	return values.Encode()
}
