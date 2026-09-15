package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestJellyfinJSONAndProgressInputsFailWithoutSideEffects(t *testing.T) {
	t.Parallel()
	handler, token, _ := jellyfinTestServer(t)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", token)
	var catalog jellyfinItems
	decodeJellyfin(t, items, &catalog)
	if len(catalog.Items) == 0 {
		t.Fatal("Jellyfin test Library Content is empty")
	}
	id := catalog.Items[0].ID

	for name, body := range map[string]string{
		"trailing document": `{"ItemId":"` + id + `","PositionTicks":420000000}{}`,
		"oversized session": `{"ItemId":"` + id + `","PositionTicks":420000000,"PlaySessionId":"` + strings.Repeat("s", 129) + `","EventSequence":1}`,
	} {
		response := jellyfinCall(t, handler, http.MethodPost, "/Sessions/Playing/Progress", body, token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d %q", name, response.Code, response.Body.String())
		}
	}
	state := jellyfinCall(t, handler, http.MethodGet, "/UserItems/"+id+"/UserData", "", token)
	if !strings.Contains(state.Body.String(), `"PlaybackPositionTicks":0`) {
		t.Fatalf("rejected progress changed state: %d %q", state.Code, state.Body.String())
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{}{}`, token); response.Code != http.StatusBadRequest {
		t.Fatalf("trailing playback request = %d %q", response.Code, response.Body.String())
	}
}

func TestJellyfinAuthenticationRejectsAmbiguousJSON(t *testing.T) {
	t.Parallel()
	handler, _, _ := jellyfinTestServer(t)
	response := jellyfinCall(t, handler, http.MethodPost, "/Users/AuthenticateByName", `{"Username":"Owner","Pw":"owner-password"}{}`, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("ambiguous authentication = %d %q", response.Code, response.Body.String())
	}
}
