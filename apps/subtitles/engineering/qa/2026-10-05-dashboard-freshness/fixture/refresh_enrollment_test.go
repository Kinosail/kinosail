package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type r16OwnerReceipt struct {
	Status int
	Body []byte
	Cookies []*http.Cookie
}

func r16OwnerResponse(ctx context.Context, target *r16Target, method, route, form string) (r16OwnerReceipt, error) {
	target.track(1)
	defer target.track(-1)
	owned, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	unlink := target.joinLifecycle(cancel)
	defer unlink()
	endpoint, err := target.endpoint(method, route)
	if err != nil || target.client == nil {
		return r16OwnerReceipt{}, errors.New("owned R16 enrollment target unavailable")
	}
	request, err := http.NewRequestWithContext(owned, method, endpoint, strings.NewReader(form))
	if err != nil {
		return r16OwnerReceipt{}, errors.New("owned R16 enrollment request unavailable")
	}
	request.Header.Set("Origin", target.origin)
	request.Header.Set("User-Agent", r16UserAgent)
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if !target.admitted(request) {
		return r16OwnerReceipt{}, errors.New("owned R16 enrollment request rejected")
	}
	response, err := target.client.Do(request)
	if err != nil {
		if response != nil {
			err = errors.Join(err, response.Body.Close())
		}
		return r16OwnerReceipt{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, r16ResponseLimit+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || len(data) > r16ResponseLimit {
		return r16OwnerReceipt{}, errors.New("owned R16 enrollment body did not settle")
	}
	return r16OwnerReceipt{Status: response.StatusCode, Body: data, Cookies: response.Cookies()}, nil
}

func r16SetupOwner(ctx context.Context, target *r16Target, password string) (string, error) {
	form := r16OwnerForm(url.Values{
		"name": {"Fictional R16 Owner"}, "password": {password},
		"totp": {"true"}, "updateMode": {"manual"},
	})
	response, err := r16OwnerResponse(ctx, target, http.MethodPost, "/setup", form)
	if err != nil {
		return "", errors.New("actual R16 Owner setup unavailable")
	}
	page := response.Body
	key := r16SetupKey.FindSubmatch(page)
	if response.Status != http.StatusOK {
		return "", errors.New("actual R16 Owner setup invalid")
	}
	if !r16SecureOwnerCookie(response.Cookies) || len(key) != 2 {
		return "", errors.New("actual R16 Owner setup authority invalid")
	}
	return string(key[1]), nil
}

func r16OwnerToken(ctx context.Context, target *r16Target, route string) (string, error) {
	response, err := r16OwnerResponse(ctx, target, http.MethodGet, route, "")
	if err != nil {
		return "", errors.New("actual R16 Owner page unavailable")
	}
	page := response.Body
	if response.Status != http.StatusOK {
		return "", errors.New("actual R16 Owner page invalid")
	}
	return r16CanonicalToken(page)
}

func r16ConfirmMFA(ctx context.Context, target *r16Target, csrf, code string) error {
	form := r16OwnerForm(url.Values{"_csrf": {csrf}, "code": {code}})
	response, err := r16OwnerResponse(ctx, target, http.MethodPost, "/account/mfa/enable", form)
	if err != nil {
		return errors.New("actual R16 MFA confirmation unavailable")
	}
	if response.Status != http.StatusSeeOther {
		return errors.New("actual R16 MFA confirmation invalid")
	}
	return nil
}
