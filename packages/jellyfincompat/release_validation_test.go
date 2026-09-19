package jellyfincompat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestInvalidRotationProfileHasNoPlaybackSideEffects(t *testing.T) {
	for _, value := range []string{"", "NaN", "-361", "361", "45", "90.0", "0|90", strings.Repeat("9", 1025)} {
		condition := map[string]any{"Property": "VideoRotation", "Condition": "Equals", "Value": value}
		body, err := json.Marshal(map[string]any{"DeviceProfile": map[string]any{"CodecProfiles": []any{map[string]any{"Type": "Video", "Conditions": []any{condition}}}}})
		if err != nil {
			t.Fatal(err)
		}
		var effects []string
		module := playbackFixture(library.Item{ID: "item", Kind: "video"}, &effects)
		response := httptest.NewRecorder()
		module.PlaybackInfo(response, playbackRequest(t, http.MethodPost, "item", string(body)))
		if response.Code != http.StatusBadRequest || strings.Join(effects, ",") != "visible" {
			t.Fatalf("value %q: status %d effects %v", value, response.Code, effects)
		}
	}
}
