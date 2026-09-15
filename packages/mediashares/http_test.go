package mediashares

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func testViews() Views {
	return NewViews("Player", "75", func(name, source string) *template.Template {
		return template.Must(template.New(name).Funcs(template.FuncMap{"icon": func(string) template.HTML { return "" }}).Parse(source))
	})
}

func executeTemplate(view *template.Template, writer http.ResponseWriter, _ *http.Request, value any) error {
	return view.Execute(writer, value)
}

func scriptHandler(content []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = writer.Write(content)
	}
}

func localError(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	http.Error(writer, message, status)
}

func TestViewsDeriveBrandsFromPlayerMarkup(t *testing.T) {
	t.Parallel()
	views := testViews()
	var output bytes.Buffer
	if err := views.Claim.Execute(&output, nil); err != nil || !strings.Contains(output.String(), "Kinosail Player") || !strings.Contains(output.String(), "app.css?v=75") || !strings.Contains(output.String(), "theme.js?v=75") {
		t.Fatalf("claim view = %q, %v", output.String(), err)
	}
	for _, invalid := range []struct{ product, asset string }{{"", "75"}, {"Player<script>", "75"}, {"Player", ""}, {"Player", "bad"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("unsafe view configuration accepted: %#v", invalid)
				}
			}()
			NewViews(invalid.product, invalid.asset, func(name, source string) *template.Template { return template.Must(template.New(name).Parse(source)) })
		}()
	}
}

func TestOwnerChoiceDefaults(t *testing.T) {
	t.Parallel()
	views := testViews()
	var output bytes.Buffer
	if err := views.Owner.Execute(&output, nil); err != nil || !strings.Contains(output.String(), `<legend>Expires</legend>`) || !strings.Contains(output.String(), `name="expires" value="3600" checked`) || !strings.Contains(output.String(), `<legend>Devices</legend>`) || !strings.Contains(output.String(), `name="devices" value="1" checked`) {
		t.Fatalf("owner choices = %q, %v", output.String(), err)
	}
}

func TestRegisteredWebLifecycle(t *testing.T) { //nolint:cyclop,funlen // One handler lifecycle proves the shared route integration.
	directory := t.TempDir()
	media := directory + "/movie.mp4"
	if err := os.WriteFile(media, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := testStore(t, time.Now(), library.Item{ID: "item", Title: "Movie", Path: media})
	mux := http.NewServeMux()
	Register(mux, store, testViews(), func(next http.Handler) http.Handler { return next }, scriptHandler, executeTemplate, localError)

	for path, contains := range map[string]string{"/share": "Opening shared media", "/static/media-share.js": "X-Kinosail-CSRF"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), contains) {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing claim = %d", missing.Code)
	}

	form := url.Values{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}}
	create := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/media-shares", strings.NewReader(form.Encode()))
	create.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	created := httptest.NewRecorder()
	mux.ServeHTTP(created, create)
	if created.Code != http.StatusOK || !strings.Contains(created.Body.String(), "/share#") {
		t.Fatalf("create = %d %q", created.Code, created.Body.String())
	}
	start := strings.Index(created.Body.String(), "/share#") + len("/share#")
	claim := created.Body.String()[start:]
	claim = claim[:strings.IndexByte(claim, '<')]

	claimRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/media-shares/claim", strings.NewReader(`{"Token":"`+claim+`","Device":"Browser"}`))
	claimResponse := httptest.NewRecorder()
	mux.ServeHTTP(claimResponse, claimRequest)
	if claimResponse.Code != http.StatusNoContent || len(claimResponse.Result().Cookies()) != 1 {
		t.Fatalf("claim = %d %q %#v", claimResponse.Code, claimResponse.Body.String(), claimResponse.Result().Cookies())
	}
	itemsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil)
	itemsRequest.AddCookie(claimResponse.Result().Cookies()[0])
	itemsResponse := httptest.NewRecorder()
	mux.ServeHTTP(itemsResponse, itemsRequest)
	if itemsResponse.Code != http.StatusOK || !strings.Contains(itemsResponse.Body.String(), "Movie") {
		t.Fatalf("items = %d %q", itemsResponse.Code, itemsResponse.Body.String())
	}
	shares, err := store.List()
	if err != nil || len(shares) != 1 {
		t.Fatalf("shares = %#v, %v", shares, err)
	}
	revoke := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/media-shares/"+shares[0].ID+"/revoke", nil)
	revoked := httptest.NewRecorder()
	mux.ServeHTTP(revoked, revoke)
	if revoked.Code != http.StatusSeeOther {
		t.Fatalf("revoke = %d %q", revoked.Code, revoked.Body.String())
	}
}

func TestAPIMutationsAreStrictAndAtomic(t *testing.T) { //nolint:cyclop // One matrix proves JSON validation and error mapping.
	t.Parallel()
	store, state := testStore(t, time.Now(), library.Item{ID: "item", Path: "movie"})
	for name, body := range map[string]string{
		"invalid": `{`, "unknown": `{"itemIds":["item"],"expiresInSeconds":3600,"maxDevices":1,"rightsAcknowledged":true,"owner":true}`,
		"policy": `{"itemIds":[],"expiresInSeconds":3600,"maxDevices":1,"rightsAcknowledged":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			store.CreateHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("create = %d %q", response.Code, response.Body.String())
			}
		})
	}
	if state.data != nil {
		t.Fatal("invalid API request persisted state")
	}
	created := httptest.NewRecorder()
	store.CreateHTTP(created, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"itemIds":["item"],"expiresInSeconds":3600,"maxDevices":1,"rightsAcknowledged":true}`)))
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), "claimToken") {
		t.Fatalf("created = %d %q", created.Code, created.Body.String())
	}
	listed := httptest.NewRecorder()
	store.ListHTTP(listed, nil)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"shares"`) {
		t.Fatalf("list = %d %q", listed.Code, listed.Body.String())
	}
	revoked := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/missing", nil)
	request.SetPathValue("id", "missing")
	store.RevokeHTTP(revoked, request)
	if revoked.Code != http.StatusNotFound {
		t.Fatalf("missing revoke = %d", revoked.Code)
	}
	claim := httptest.NewRecorder()
	store.ClaimHTTP(claim, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"Token":"short","Device":"Browser"}`)))
	if claim.Code != http.StatusNotFound {
		t.Fatalf("invalid claim = %d %q", claim.Code, claim.Body.String())
	}
}

func TestWebFormRejectsUnsafeFieldsAndBounds(t *testing.T) {
	t.Parallel()
	for _, form := range []url.Values{
		{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"1"}},
		{"itemIds": {"item"}, "expires": {"3600", "7200"}, "devices": {"1"}, "rightsAcknowledged": {"true"}},
		{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}, "unknown": {"true"}},
		{"itemIds": {strings.Repeat("x", 257)}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), request); err == nil {
			t.Fatalf("unsafe form accepted: %#v", form)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/?query=true", strings.NewReader(""))
	if _, _, _, err := decodeCreateForm(httptest.NewRecorder(), request); err == nil {
		t.Fatal("query form accepted")
	}
}

func TestRegisterRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("missing HTTP dependencies were accepted")
		}
	}()
	Register(nil, nil, Views{}, nil, nil, nil, nil)
}
