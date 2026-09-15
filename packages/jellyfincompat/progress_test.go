package jellyfincompat

import (
	"errors"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProgressAndFavoriteSemanticsUsePlayerCopy(t *testing.T) { //nolint:cyclop,funlen,gocognit // One table covers every canonical operation outcome.
	t.Parallel()
	item := library.Item{ID: "1234567890abcdef"}
	data := ProjectUserDataDTO(item, UserState{Seconds: 1.25, Watched: true}, true, nil)
	if data["PlaybackPositionTicks"] != int64(12_500_000) || data["Played"] != true || data["IsFavorite"] != true {
		t.Fatalf("user data = %#v", data)
	}
	projected := ProjectUserDataDTO(library.Item{ID: item.ID, Kind: "video"}, UserState{Seconds: 2}, false, func(seconds float64) float64 { return seconds / 2 })
	if projected["PlaybackPositionTicks"] != int64(10_000_000) {
		t.Fatalf("projected user data = %#v", projected)
	}

	invalid := errors.New("invalid")
	tests := []struct {
		name     string
		found    bool
		ticks    int64
		accepted bool
		err      error
		status   int
		message  string
	}{
		{"missing", false, 0, true, nil, http.StatusBadRequest, "invalid playback state"},
		{"negative", true, -1, true, nil, http.StatusBadRequest, "invalid playback state"},
		{"invalid", true, 0, true, invalid, http.StatusBadRequest, "invalid playback state"},
		{"failed", true, 0, true, errors.New("disk"), http.StatusInternalServerError, "could not save playback state"},
		{"stale", true, 0, false, nil, http.StatusConflict, "playback state is stale"},
		{"saved", true, 20_000_000, true, nil, 0, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called, seconds := false, 0.0
			operationErr := SaveProgress(test.found, test.ticks, func(value float64) (bool, error) {
				called, seconds = true, value
				return test.accepted, test.err
			}, func(err error) bool { return errors.Is(err, invalid) })
			if test.status == 0 {
				if operationErr != nil || !called || seconds != 2 {
					t.Fatalf("saved = %#v called=%v seconds=%v", operationErr, called, seconds)
				}
				return
			}
			if operationErr == nil || operationErr.Status != test.status || operationErr.Message != test.message {
				t.Fatalf("error = %#v", operationErr)
			}
		})
	}

	for _, operation := range []struct {
		name string
		run  func(bool, func() error) *OperationError
		copy string
	}{
		{"favorite", SaveFavorite, "could not save favorite"},
		{"played", SavePlayed, "could not save playback state"},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if result := operation.run(false, func() error { t.Fatal("save called"); return nil }); result.Status != http.StatusNotFound {
				t.Fatalf("not found = %#v", result)
			}
			if result := operation.run(true, func() error { return errors.New("failed") }); result.Status != http.StatusInternalServerError || result.Message != operation.copy {
				t.Fatalf("failed = %#v", result)
			}
			if result := operation.run(true, func() error { return nil }); result != nil {
				t.Fatalf("success = %#v", result)
			}
		})
	}
}
