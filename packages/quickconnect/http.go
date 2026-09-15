package quickconnect

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

// RequestBodyMaximum bounds every Quick Connect request body.
const RequestBodyMaximum = 4 << 10

var errHTTPRequest = errors.New("Quick Connect request is invalid") //nolint:staticcheck // Product term starts with a capital.

// Start creates one strictly parsed pending authorization request.
func (application *Application) Start(writer http.ResponseWriter, request *http.Request) {
	if !application.limiters.Requests.Allow(httpguard.RemoteHost(request.RemoteAddr), 60) {
		tooMany(writer, "too many Quick Connect requests")
		return
	}
	device, ok := readStartDevice(writer, request)
	if !ok {
		return
	}
	if !application.limiters.Starts.Allow(httpguard.RemoteHost(request.RemoteAddr), 20) {
		tooMany(writer, "too many Quick Connect requests")
		return
	}
	secret, pending, err := application.broker.Create(Request{
		Device: application.adapter.CleanDevice(device), Client: "Kinosail", Version: "1", Numeric: true,
		Remote: identitycore.RemoteRequest(request),
	})
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
			return
		}
		apihttp.Error(writer, errors.New("too many Quick Connect requests"), http.StatusTooManyRequests) //nolint:staticcheck // Product term starts with a capital.
		return
	}
	apihttp.WriteJSON(writer, map[string]string{"code": pending.Code, "secret": secret}, http.StatusCreated)
}

// Poll returns pending state or consumes one approved request.
func (application *Application) Poll(writer http.ResponseWriter, request *http.Request) {
	if !application.limiters.Polls.Allow(httpguard.RemoteHost(request.RemoteAddr), 120) {
		tooMany(writer, "too many Quick Connect polls")
		return
	}
	secret, ok := readPollSecret(writer, request)
	if !ok {
		return
	}
	token, _, err := application.Consume(secret)
	if errors.Is(err, ErrPending) {
		apihttp.WriteJSON(writer, map[string]string{"status": "pending"}, http.StatusAccepted)
		return
	}
	if err != nil {
		apihttp.NotFound(writer)
		return
	}
	expires := 2592000
	if identitycore.RemoteRequest(request) {
		expires = 28800
	}
	apihttp.WriteJSON(writer, map[string]any{"token": token, "expiresIn": expires}, http.StatusCreated)
}

// Page renders the product Quick Connect page.
func (application *Application) Page(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	page := application.page
	if request.URL.RawQuery != "" || request.URL.ForceQuery {
		code, ok := ReadLinkCode(request)
		if !ok {
			application.adapter.Error(writer, request, ErrInvalidRequest.Error(), http.StatusBadRequest)
			return
		}
		pending, err := application.broker.Preview(code, application.viewer(request))
		if err != nil {
			application.adapter.Error(writer, request, "This code expired or was already used. Request a new code on your TV.", http.StatusBadRequest)
			return
		}
		page.Code, page.Device, page.AutoSubmit = pending.Code, pending.Device, false
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = application.adapter.Render(writer, request, page)
}

// ApprovePage validates and applies one browser approval.
func (application *Application) ApprovePage(writer http.ResponseWriter, request *http.Request) {
	if identitycore.RemoteRequest(request) {
		application.adapter.Error(writer, request, "not found", http.StatusNotFound)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, RequestBodyMaximum)
	if !strictRequiredForm(request, "code") {
		application.adapter.Error(writer, request, ErrInvalidRequest.Error(), http.StatusBadRequest)
		return
	}
	code := request.PostForm.Get("code")
	if !validApprovalValue(code) {
		application.adapter.Error(writer, request, ErrInvalidRequest.Error(), http.StatusBadRequest)
		return
	}
	if err := application.approveRequest(request, code); err != nil {
		application.adapter.Error(writer, request, err.Error(), approvalStatus(err))
		return
	}
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

// ApproveAPI validates and applies one versioned API approval.
func (application *Application) ApproveAPI(writer http.ResponseWriter, request *http.Request) {
	if identitycore.RemoteRequest(request) {
		apihttp.NotFound(writer)
		return
	}
	if !httpguard.EmptyMutationRequest(writer, request) {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return
	}
	code := request.PathValue("code")
	if !validApprovalValue(code) {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return
	}
	if err := application.approveRequest(request, code); err != nil {
		apihttp.Error(writer, err, approvalStatus(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) approveRequest(request *http.Request, code string) error {
	if !application.limiters.Requests.Allow("approve:"+httpguard.RemoteHost(request.RemoteAddr), 30) {
		return ErrCapacity
	}
	profile := application.adapter.Current(request)
	strong := application.adapter.RecentlyAuthenticated(request, ApprovalMaximumAge)
	return application.Approve(profile, code, strong)
}

func readStartDevice(writer http.ResponseWriter, request *http.Request) (string, bool) {
	request.Body = http.MaxBytesReader(writer, request.Body, RequestBodyMaximum)
	if request.URL.RawQuery != "" {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return "", false
	}
	contentType := request.Header.Get("Content-Type")
	mediaType := ""
	if contentType != "" {
		var err error
		mediaType, _, err = mime.ParseMediaType(contentType)
		if err != nil {
			apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
			return "", false
		}
	}
	switch {
	case mediaType == "application/json":
		return readStringJSON(writer, request, "device", false)
	case mediaType == "":
		body, err := io.ReadAll(request.Body)
		if err == nil && len(body) == 0 {
			return "", true
		}
	case mediaType == "application/x-www-form-urlencoded" && strictForm(request, "device"):
		return request.PostForm.Get("device"), true
	}
	apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
	return "", false
}

func readPollSecret(writer http.ResponseWriter, request *http.Request) (string, bool) {
	request.Body = http.MaxBytesReader(writer, request.Body, RequestBodyMaximum)
	if request.URL.RawQuery != "" {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return "", false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	switch {
	case err != nil:
	case mediaType == "application/json":
		return readStringJSON(writer, request, "secret", true)
	case mediaType == "application/x-www-form-urlencoded" && strictRequiredForm(request, "secret"):
		return request.PostForm.Get("secret"), true
	}
	apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
	return "", false
}

func readStringJSON(writer http.ResponseWriter, request *http.Request, wanted string, required bool) (string, bool) { //nolint:cyclop // Token parsing rejects duplicate and unknown fields before state changes.
	invalid := func() (string, bool) {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return "", false
	}
	decoder := json.NewDecoder(request.Body)
	token, err := decoder.Token()
	delimiter, object := token.(json.Delim)
	if err != nil || !object || delimiter != '{' {
		return invalid()
	}
	value, found := "", false
	for decoder.More() {
		token, err = decoder.Token()
		field, stringField := token.(string)
		if err != nil || !stringField || !strings.EqualFold(field, wanted) || found || decoder.Decode(&value) != nil {
			return invalid()
		}
		found = true
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || required && !found {
		return invalid()
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return invalid()
	}
	return value, true
}

func strictForm(request *http.Request, keys ...string) bool {
	if request.URL.RawQuery != "" || !httpguard.FormEncoded(request) || request.ParseForm() != nil || !httpguard.OnlyFormKeys(request.PostForm, keys...) {
		return false
	}
	for _, values := range request.PostForm {
		if len(values) != 1 {
			return false
		}
	}
	return true
}

func strictRequiredForm(request *http.Request, key string) bool {
	return strictForm(request, key) && len(request.PostForm[key]) == 1 && request.PostForm.Get(key) != ""
}

func validApprovalValue(value string) bool {
	return len(value) <= 16 && strings.TrimSpace(value) != ""
}

func tooMany(writer http.ResponseWriter, message string) {
	writer.Header().Set("Retry-After", "60")
	apihttp.Error(writer, errors.New(message), http.StatusTooManyRequests)
}
