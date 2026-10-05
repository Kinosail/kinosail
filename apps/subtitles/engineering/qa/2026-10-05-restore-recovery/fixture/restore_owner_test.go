package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func enrollOwner(t *testing.T, target *restoreTarget) (*http.Client, string, error) {
	client := target.privateClient(15 * time.Second)
	t.Cleanup(client.CloseIdleConnections)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", errors.New("cookiejar unavailable")
	}
	client.Jar = jar
	password, err := restorePassword()
	if err != nil {
		return nil, "", err
	}
	key, err := setupOwner(t, target, client, password)
	if err != nil {
		return nil, "", err
	}
	csrf, err := enrollmentToken(t, target, client)
	if err != nil {
		return nil, "", err
	}
	code := servertest.TestTOTP(t, key, time.Now())
	if err = confirmOwnerMFA(t, target, client, csrf, code); err != nil {
		return nil, "", err
	}
	current, err := currentOwnerToken(t, target, client)
	if err != nil {
		return nil, "", err
	}
	return client, current, nil
}

func restorePassword() (string, error) {
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", errors.New("credential generation failed")
	}
	return hex.EncodeToString(secretBytes) + "Aa7!", nil
}

func restoreResponseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
