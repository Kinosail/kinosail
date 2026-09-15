package trustedhttps

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const settingsFormMaxBody = 16 << 10

// LimitSettingsForms applies the form bound before middleware can parse the body.
func LimitSettingsForms(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && (request.URL.Path == "/settings/trusted-https" || request.URL.Path == "/onboarding/trusted-https") && request.Body != nil {
			request.Body = http.MaxBytesReader(writer, request.Body, settingsFormMaxBody)
		}
		next.ServeHTTP(writer, request)
	})
}

// SettingsInput contains the fields accepted by the trusted HTTPS operation.
type SettingsInput struct {
	Provider, Domain, Token, Address string
	TermsAccepted                    bool
}

// SettingsForm connects the shared form flow to an application's settings operation.
type SettingsForm struct {
	Save                       func(SettingsInput) error
	Managed, Conflict, Storage error
	Error                      func(http.ResponseWriter, *http.Request, string, int)
	Redirect                   string
}

// ServeHTTP validates input, saves settings, and maps the operation result.
func (form SettingsForm) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	input, err := ParseSettingsForm(writer, request)
	if err != nil {
		form.Error(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err := form.Save(input); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, form.Managed) || errors.Is(err, form.Conflict) {
			status = http.StatusConflict
		} else if errors.Is(err, form.Storage) {
			status = http.StatusInternalServerError
		}
		form.Error(writer, request, err.Error(), status)
		return
	}
	http.Redirect(writer, request, form.Redirect, http.StatusSeeOther)
}

// ParseSettingsForm decodes Player's bounded trusted HTTPS form before any save.
func ParseSettingsForm(writer http.ResponseWriter, request *http.Request) (SettingsInput, error) {
	if err := httpguard.DecodeForm(writer, request, settingsFormMaxBody, "provider", "domain", "token", "address", "termsAccepted", "_csrf"); err != nil {
		return SettingsInput{}, errors.New("invalid trusted HTTPS form")
	}
	form := request.PostForm
	for _, key := range []string{"domain", "token", "address", "termsAccepted"} {
		if len(form[key]) != 1 {
			return SettingsInput{}, errors.New("complete every trusted HTTPS field")
		}
	}
	if form.Get("termsAccepted") != "true" {
		return SettingsInput{}, errors.New("complete every trusted HTTPS field")
	}
	return SettingsInput{Provider: form.Get("provider"), Domain: form.Get("domain"), Token: form.Get("token"), Address: form.Get("address"), TermsAccepted: true}, nil
}
