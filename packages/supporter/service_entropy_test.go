package supporter

import (
	"crypto/rand"
	"errors"
	"testing"
)

type failedEntropy struct{}

func (failedEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func TestPrepareLeavesStateUnchangedWhenEntropyFails(t *testing.T) {
	previous := rand.Reader
	rand.Reader = failedEntropy{}
	t.Cleanup(func() { rand.Reader = previous })
	state := State{}
	prepared, err := (&Service{}).Prepare(state)
	if !errors.Is(err, ErrUnavailable) || prepared.InstallationKey != "" || state.InstallationKey != "" {
		t.Fatalf("failed preparation created an installation identity: state=%+v err=%v", prepared, err)
	}
}
