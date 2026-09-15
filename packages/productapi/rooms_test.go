package productapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

type roomService struct {
	item                        library.Item
	found, created              bool
	visible, viewers, creations int
	viewer                      identitycore.Profile
}

func (service *roomService) Visible(_ *http.Request, _ string) (library.Item, bool) {
	service.visible++
	return service.item, service.found
}

func (service *roomService) Viewer(*http.Request) identitycore.Profile {
	service.viewers++
	return service.viewer
}

func (service *roomService) Create(viewer identitycore.Profile, media string, seconds float64) (string, bool) {
	service.creations++
	if viewer.ID != service.viewer.ID || media != service.item.ID || seconds != 12.5 {
		return "", false
	}
	return "room", service.created
}

func roomRequest(t *testing.T, service RoomService, body, content, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/rooms"+query, strings.NewReader(body))
	if content != "" {
		request.Header.Set("Content-Type", content)
	}
	response := httptest.NewRecorder()
	CreateRoom(service).ServeHTTP(response, request)
	return response
}

func TestCreateRoomValidatesBeforeSideEffects(t *testing.T) {
	t.Parallel()
	valid := `{"media":"movie","seconds":12.5}`
	for name, test := range map[string]struct{ body, content, query string }{
		"query":            {valid, "application/json", "?force=true"},
		"missing type":     {valid, "", ""},
		"malformed":        {`{`, "application/json", ""},
		"unknown":          {`{"media":"movie","seconds":12.5,"other":true}`, "application/json", ""},
		"duplicate":        {`{"media":"movie","media":"other","seconds":12.5}`, "application/json", ""},
		"missing media":    {`{"seconds":12.5}`, "application/json", ""},
		"oversized media":  {`{"media":"` + strings.Repeat("x", maximumItemID+1) + `","seconds":12.5}`, "application/json", ""},
		"negative seconds": {`{"media":"movie","seconds":-1}`, "application/json", ""},
		"oversized body":   {`{"media":"` + strings.Repeat("x", 1<<20) + `"}`, "application/json", ""},
	} {
		t.Run(name, func(t *testing.T) {
			service := &roomService{found: true, created: true, item: library.Item{ID: "movie", Kind: "video"}}
			response := roomRequest(t, service, test.body, test.content, test.query)
			if response.Code != http.StatusBadRequest || service.visible != 0 || service.viewers != 0 || service.creations != 0 {
				t.Fatalf("response = %d %q, service = %#v", response.Code, response.Body.String(), service)
			}
		})
	}
}

func TestCreateRoomPreservesVisibilityAndCapacityOutcomes(t *testing.T) {
	t.Parallel()
	for name, service := range map[string]*roomService{
		"missing":  {item: library.Item{ID: "movie", Kind: "video"}},
		"photo":    {found: true, item: library.Item{ID: "movie", Kind: "photo"}},
		"capacity": {found: true, item: library.Item{ID: "movie", Kind: "video"}, viewer: identitycore.Profile{ID: "viewer"}},
		"success":  {found: true, created: true, item: library.Item{ID: "movie", Kind: "video"}, viewer: identitycore.Profile{ID: "viewer"}},
	} {
		t.Run(name, func(t *testing.T) {
			response := roomRequest(t, service, `{"media":"movie","seconds":12.5}`, "application/json; charset=utf-8", "")
			want := map[string]int{"missing": http.StatusBadRequest, "photo": http.StatusBadRequest, "capacity": http.StatusTooManyRequests, "success": http.StatusCreated}[name]
			if response.Code != want {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if (name == "missing" || name == "photo") && (service.viewers != 0 || service.creations != 0) {
				t.Fatalf("visibility failure had side effects: %#v", service)
			}
			if name == "success" && !strings.Contains(response.Body.String(), `"watch":"/watch/movie?room=room"`) {
				t.Fatalf("success body = %q", response.Body.String())
			}
		})
	}
}
