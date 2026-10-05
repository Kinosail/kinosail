package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func enrollOwner(t *testing.T, f *fixture) (*http.Client, string, error) {
	client := f.privateClient(15 * time.Second)
	t.Cleanup(client.CloseIdleConnections)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", errors.New("cookiejar unavailable")
	}
	client.Jar = jar
	password, err := fixturePassword()
	if err != nil {
		return nil, "", err
	}
	key, err := setupOwner(t, f, client, password)
	if err != nil {
		return nil, "", err
	}
	csrf, err := enrollmentToken(t, f, client)
	if err != nil {
		return nil, "", err
	}
	code := servertest.TestTOTP(t, key, time.Now())
	if err = confirmOwnerMFA(t, f, client, csrf, code); err != nil {
		return nil, "", err
	}
	current, err := currentOwnerToken(t, f, client)
	if err != nil {
		return nil, "", err
	}
	return client, current, nil
}

func fixturePassword() (string, error) {
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", errors.New("credential generation failed")
	}
	return hex.EncodeToString(secretBytes) + "Aa7!", nil
}

func controlResponseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

type controlDiagnostic struct {
	stage                             string
	responseStatus                    int
	transport, read, bounded, decoded bool
}

func controlRoute(path string) string {
	switch {
	case path == "/api/v1/subtitle-library?view=library":
		return "catalog"
	case strings.HasSuffix(path, "/inspect?language=en"):
		return "inspection"
	case strings.HasSuffix(path, "/preview"):
		return "preview"
	case path == "/api/v1/subtitle-operations":
		return "prepare"
	default:
		return "unknown"
	}
}

func (d *controlDiagnostic) emit(t *testing.T, returned int) {
	t.Logf("R06_ROUTE %s %d %d %t %t %t %t", d.stage, d.responseStatus, returned,
		d.transport, d.read, d.bounded, d.decoded)
}
