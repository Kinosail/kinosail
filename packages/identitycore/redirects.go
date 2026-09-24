package identitycore

import (
	"net/http"
	"net/url"
	"strings"
)

// StepUpLoginPath returns the safe owner destination for a step-up request.
func StepUpLoginPath(request *http.Request) string {
	next := "/"
	if request != nil && request.URL != nil {
		switch {
		case strings.HasPrefix(request.URL.Path, "/settings/management"):
			next = "/settings/management"
		case request.URL.Path == "/settings/backup":
			next = "/settings/backups"
		case request.URL.Path == "/settings/encrypted-backup", strings.HasPrefix(request.URL.Path, "/settings/backups/"):
			next = "/settings/backups"
		case strings.HasPrefix(request.URL.Path, "/settings/"):
			next = "/settings"
		case strings.HasPrefix(request.URL.Path, "/account/"):
			next = "/account"
		}
	}
	return "/login?stepup=1&next=" + url.QueryEscape(next)
}

// SafeLoginReturn accepts only bounded same-origin relative destinations.
func SafeLoginReturn(raw string) string {
	if raw == "" || len(raw) > 2048 || strings.HasPrefix(raw, "//") {
		return "/"
	}
	raw, _, _ = strings.Cut(raw, "#")
	target, err := url.Parse(raw)
	if err != nil || target.IsAbs() || target.Hostname() != "" || !strings.HasPrefix(target.Path, "/") || strings.HasPrefix(target.Path, "//") || strings.Contains(target.Path, `\`) {
		return "/"
	}
	return target.RequestURI()
}

// PasskeyOfferPath returns the account path that offers a passkey for next.
func PasskeyOfferPath(next string) string {
	return "/account?passkey=offer&next=" + url.QueryEscape(next)
}
