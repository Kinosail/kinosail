package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var setupKey = regexp.MustCompile("<code>([A-Z2-7]{32})</code>")
var setupCSRF = regexp.MustCompile("name=\"_csrf\" value=\"([A-Za-z0-9_-]{43})\"")
var pageCSRF = regexp.MustCompile("name=\"kinosail-csrf\" content=\"([A-Za-z0-9_-]{43})\"")

func ownerResponse(t *testing.T, f *fixture, client *http.Client, stage, method, path, form string) (*http.Response, error) {
	request, err := f.privateRequest(t.Context(), method, path, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if !f.admittedPrivateRequest(request) { return nil, errors.New("private request boundary unavailable") }
	response, err := client.Do(request)
	t.Logf("R06_OWNER_REQUEST %s %d %t", stage, controlResponseStatus(response), err == nil)
	if err != nil { f.closeFailedPrivateResponse(response) }
	return response, err
}

func setupOwner(t *testing.T, f *fixture, client *http.Client, password string) (string, error) {
	form := "name=Fictional+Owner&password=" + password + "&totp=true&updateMode=manual"
	response, err := ownerResponse(t, f, client, "setup", http.MethodPost, "/setup", form)
	if err != nil {
		return "", errors.New("setup unavailable")
	}
	body, err := readPrivateResponse(response)
	secure := secureOwnerCookie(response.Cookies())
	key := setupKey.FindSubmatch(body)
	csrf := setupCSRF.FindSubmatch(body)
	t.Logf("R06_OWNER_SETUP %d %t %t %t %t", response.StatusCode, err == nil, secure, len(key) == 2, len(csrf) == 2)
	if response.StatusCode != http.StatusOK || err != nil || len(body) > privateResponseLimit {
		return "", errors.New("setup response invalid")
	}
	if !secure {
		return "", errors.New("setup response invalid")
	}
	if len(key) != 2 {
		return "", errors.New("setup response invalid")
	}
	return string(key[1]), nil
}

func secureOwnerCookie(cookies []*http.Cookie) bool {
	for _, cookie := range cookies {
		if cookie.Name != "__Host-kinosail_subtitles_session" {
			continue
		}
		if cookie.Secure && cookie.HttpOnly && cookie.SameSite == http.SameSiteStrictMode && cookie.Path == "/" {
			return true
		}
	}
	return false
}

func enrollmentToken(t *testing.T, f *fixture, client *http.Client) (string, error) {
	// Setup sets the response cookie; request-bound CSRF comes from a real authenticated page.
	response, err := ownerResponse(t, f, client, "enrollment", http.MethodGet, "/account", "")
	if err != nil {
		return "", errors.New("authenticated enrollment page unavailable")
	}
	page, readErr := readPrivateResponse(response)
	tokens := pageCSRF.FindAllSubmatch(page, -1)
	t.Logf("R06_OWNER_ENROLLMENT %d %t %t %t", response.StatusCode, readErr == nil, len(page) <= privateResponseLimit, len(tokens) == 1)
	if response.StatusCode != http.StatusOK || readErr != nil || len(page) > privateResponseLimit {
		return "", errors.New("authenticated enrollment CSRF boundary unavailable")
	}
	if len(tokens) != 1 {
		return "", errors.New("authenticated enrollment CSRF boundary unavailable")
	}
	if len(tokens[0]) != 2 {
		return "", errors.New("authenticated enrollment CSRF boundary unavailable")
	}
	return string(tokens[0][1]), nil
}

func confirmOwnerMFA(t *testing.T, f *fixture, client *http.Client, csrf, code string) error {
	form := "_csrf=" + csrf + "&code=" + code
	response, err := ownerResponse(t, f, client, "mfa", http.MethodPost, "/account/mfa/enable", form)
	if err != nil {
		return errors.New("MFA confirmation unavailable")
	}
	data, readErr := readPrivateResponse(response)
	if readErr != nil || len(data) > privateResponseLimit {
		return errors.New("MFA confirmation unavailable")
	}
	if response.StatusCode != http.StatusSeeOther {
		return errors.New("MFA confirmation rejected")
	}
	return nil
}

func currentOwnerToken(t *testing.T, f *fixture, client *http.Client) (string, error) {
	response, err := ownerResponse(t, f, client, "current", http.MethodGet, "/?view=library", "")
	if err != nil {
		return "", errors.New("current Owner page unavailable")
	}
	page, readErr := readPrivateResponse(response)
	tokens := pageCSRF.FindAllSubmatch(page, -1)
	t.Logf("R06_OWNER_CURRENT %d %t %t", response.StatusCode, readErr == nil, len(tokens) == 1)
	if response.StatusCode != http.StatusOK || readErr != nil || len(page) > privateResponseLimit {
		return "", errors.New("current Owner/CSRF boundary unavailable")
	}
	if len(tokens) != 1 {
		return "", errors.New("current Owner/CSRF boundary unavailable")
	}
	if len(tokens[0]) != 2 {
		return "", errors.New("current Owner/CSRF boundary unavailable")
	}
	return string(tokens[0][1]), nil
}
