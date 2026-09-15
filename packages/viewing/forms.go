package viewing

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
)

const maximumFormBytes = 1 << 20

// ErrPreviewRender is the stable public error for an app-owned preview template failure.
var ErrPreviewRender = errors.New("viewing activity preview failed")

// WebPreviewPage describes the shared route-specific preview navigation policy.
type WebPreviewPage struct {
	BackPath, BackLabel, Step, ApplyPath string
	AllowSync                            bool
}

// WebPreview is the complete shared model rendered by an app-owned template.
type WebPreview struct {
	Preview
	WebPreviewPage
}

// PreviewPage selects settings or onboarding navigation without changing preview data.
func PreviewPage(path string) WebPreviewPage {
	if path == "/onboarding/viewing-imports/preview" {
		return WebPreviewPage{BackPath: "/onboarding/migrate", BackLabel: "Connection", Step: "Step 4 of 4", ApplyPath: "/onboarding/viewing-imports/apply"}
	}
	return WebPreviewPage{BackPath: "/settings#viewing-imports", BackLabel: "Settings", Step: "Migration preview", ApplyPath: "/settings/viewing-imports/apply", AllowSync: true}
}

// ServeWebPreview validates and prepares one preview before invoking the renderer.
func ServeWebPreview(ctx context.Context, writer http.ResponseWriter, request *http.Request, preview func(context.Context, Input) (Preview, error), render func(WebPreview) error) (int, error) {
	result, status, err := PreviewForm(ctx, request, preview)
	if err != nil {
		return status, err
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := render(WebPreview{result, PreviewPage(request.URL.Path)}); err != nil {
		return http.StatusInternalServerError, ErrPreviewRender
	}
	return http.StatusOK, nil
}

// ParseImportForm strictly parses one source import request.
func ParseImportForm(request *http.Request) (Input, error) {
	if err := parseForm(request, "source", "url", "token", "sourceUser", "profileId", "overwriteExisting"); err != nil {
		return Input{}, errors.New("invalid viewing import form")
	}
	source, sourceOK := optionalValue(request.PostForm, "source", 16)
	endpoint, endpointOK := optionalValue(request.PostForm, "url", 2048)
	token, tokenOK := optionalValue(request.PostForm, "token", 4096)
	sourceUser, sourceUserOK := optionalValue(request.PostForm, "sourceUser", 512)
	profileID, profileOK := optionalValue(request.PostForm, "profileId", 128)
	overwrite, overwriteOK := optionalValue(request.PostForm, "overwriteExisting", 4)
	for _, valid := range []bool{sourceOK, endpointOK, tokenOK, sourceUserOK, profileOK, overwriteOK} {
		if !valid {
			return Input{}, errors.New("invalid viewing import form")
		}
	}
	if overwrite != "" && overwrite != "true" {
		return Input{}, errors.New("overwriteExisting must be true when provided")
	}
	return Input{Source: source, URL: endpoint, Token: token, SourceUser: sourceUser, ProfileID: profileID, Overwrite: overwrite == "true"}, nil
}

// PreviewForm validates a source form before invoking any remote or destination read.
func PreviewForm(ctx context.Context, request *http.Request, preview func(context.Context, Input) (Preview, error)) (Preview, int, error) {
	input, err := ParseImportForm(request)
	if err != nil {
		return Preview{}, http.StatusBadRequest, err
	}
	result, err := preview(ctx, input)
	if err != nil {
		return Preview{}, http.StatusBadRequest, err
	}
	return result, http.StatusOK, nil
}

// ParseActionForm strictly parses one preview or sync identifier.
func ParseActionForm(request *http.Request) (string, error) {
	if err := parseForm(request, "id"); err != nil {
		return "", errors.New("invalid viewing activity form")
	}
	id, ok := requiredValue(request.PostForm, "id", 128)
	if !ok || !ValidID(id) {
		return "", errors.New("invalid viewing activity form")
	}
	return id, nil
}

// ApplyForm validates an action identifier before invoking a mutating preview operation.
func ApplyForm(request *http.Request, apply func(string) (Summary, error)) (Summary, int, error) {
	id, err := ParseActionForm(request)
	if err != nil {
		return Summary{}, http.StatusBadRequest, err
	}
	result, err := apply(id)
	if err != nil {
		return Summary{}, http.StatusConflict, err
	}
	return result, http.StatusOK, nil
}

// ParseSyncForm strictly parses one preview identifier and sync interval.
func ParseSyncForm(request *http.Request) (string, string, error) {
	if err := parseForm(request, "id", "interval"); err != nil {
		return "", "", errors.New("invalid viewing activity form")
	}
	id, idOK := requiredValue(request.PostForm, "id", 128)
	interval, intervalOK := requiredValue(request.PostForm, "interval", 3)
	if !idOK || !ValidID(id) || !intervalOK {
		return "", "", errors.New("invalid viewing activity form")
	}
	return id, interval, nil
}

// ValidID reports whether an import preview or sync identifier is bounded base32.
func ValidID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, character := range id {
		if (character < 'A' || character > 'Z') && (character < '2' || character > '7') {
			return false
		}
	}
	return true
}

func parseForm(request *http.Request, keys ...string) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" {
		return errors.New("invalid form")
	}
	request.Body = http.MaxBytesReader(nil, request.Body, maximumFormBytes)
	if err := request.ParseForm(); err != nil {
		return err
	}
	for key := range request.PostForm {
		if !slices.Contains(keys, key) {
			return errors.New("invalid form")
		}
	}
	return nil
}

func optionalValue(values url.Values, key string, limit int) (string, bool) {
	selected, found := values[key]
	if !found {
		return "", true
	}
	return boundedValue(selected, limit, false)
}

func requiredValue(values url.Values, key string, limit int) (string, bool) {
	return boundedValue(values[key], limit, true)
}

func boundedValue(values []string, limit int, required bool) (string, bool) {
	if len(values) != 1 || len(values[0]) > limit || required && values[0] == "" {
		return "", false
	}
	return values[0], true
}
