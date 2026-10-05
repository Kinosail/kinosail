package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func enrollOwner(t *testing.T, f *fixture) (*http.Client, string, error) {
	client := *f.tls.Client()
	client.Jar, _ = cookiejar.New(nil)
	client.Timeout = 15 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, "", errors.New("credential generation failed")
	}
	password := hex.EncodeToString(secretBytes) + "Aa7!"
	setup := "name=Fictional+Owner&password=" + password + "&totp=true&updateMode=manual"
	request, _ := http.NewRequest(http.MethodPost, f.origin+"/setup", strings.NewReader(setup))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", f.origin)
	request.Header.Set("User-Agent", "R06-private-control")
	response, err := client.Do(request)
	t.Logf("R06_OWNER_REQUEST setup %d %t", controlResponseStatus(response), err == nil)
	if err != nil {
		return nil, "", errors.New("setup unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	response.Body.Close()
	secure := false
	for _, cookie := range response.Cookies() {
		secure = secure || cookie.Name == "__Host-kinosail_subtitles_session" &&
			cookie.Secure && cookie.HttpOnly && cookie.SameSite == http.SameSiteStrictMode && cookie.Path == "/"
	}
	key := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindSubmatch(body)
	csrf := regexp.MustCompile(`name="_csrf" value="([A-Za-z0-9_-]{43})"`).FindSubmatch(body)
	t.Logf("R06_OWNER_SETUP %d %t %t %t %t", response.StatusCode, err == nil, secure, len(key) == 2, len(csrf) == 2)
	if response.StatusCode != 200 || err != nil || !secure || len(key) != 2 {
		return nil, "", errors.New("setup response invalid")
	}
	// Setup sets the response cookie; request-bound CSRF comes from a real authenticated page.
	enrollment, err := client.Get(f.origin + "/account")
	t.Logf("R06_OWNER_REQUEST enrollment %d %t", controlResponseStatus(enrollment), err == nil)
	if err != nil {
		return nil, "", errors.New("authenticated enrollment page unavailable")
	}
	enrollmentPage, readErr := io.ReadAll(io.LimitReader(enrollment.Body, 65537))
	enrollment.Body.Close()
	enrollmentCSRF := regexp.MustCompile(`name="kinosail-csrf" content="([A-Za-z0-9_-]{43})"`).FindAllSubmatch(enrollmentPage, -1)
	t.Logf("R06_OWNER_ENROLLMENT %d %t %t %t", enrollment.StatusCode, readErr == nil, len(enrollmentPage) <= 65536, len(enrollmentCSRF) == 1)
	if enrollment.StatusCode != 200 || readErr != nil || len(enrollmentPage) > 65536 || len(enrollmentCSRF) != 1 {
		return nil, "", errors.New("authenticated enrollment CSRF boundary unavailable")
	}
	csrf = enrollmentCSRF[0]
	code := servertest.TestTOTP(t, string(key[1]), time.Now())
	input := "_csrf=" + string(csrf[1]) + "&code=" + code
	request, _ = http.NewRequest(http.MethodPost, f.origin+"/account/mfa/enable", strings.NewReader(input))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", f.origin)
	request.Header.Set("User-Agent", "R06-private-control")
	response, err = client.Do(request)
	t.Logf("R06_OWNER_REQUEST mfa %d %t", controlResponseStatus(response), err == nil)
	if err != nil {
		return nil, "", errors.New("MFA confirmation unavailable")
	}
	response.Body.Close()
	if response.StatusCode != 303 {
		return nil, "", errors.New("MFA confirmation rejected")
	}
	current, err := client.Get(f.origin + "/?view=library")
	t.Logf("R06_OWNER_REQUEST current %d %t", controlResponseStatus(current), err == nil)
	if err != nil {
		return nil, "", errors.New("current Owner page unavailable")
	}
	page, readErr := io.ReadAll(io.LimitReader(current.Body, 65537))
	current.Body.Close()
	currentCSRF := regexp.MustCompile(`name="kinosail-csrf" content="([A-Za-z0-9_-]{43})"`).FindSubmatch(page)
	t.Logf("R06_OWNER_CURRENT %d %t %t", current.StatusCode, readErr == nil, len(currentCSRF) == 2)
	if current.StatusCode != 200 || readErr != nil || len(currentCSRF) != 2 {
		return nil, "", errors.New("current Owner/CSRF boundary unavailable")
	}
	return &client, string(currentCSRF[1]), nil
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
