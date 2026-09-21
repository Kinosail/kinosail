package appcli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerRecoveryDispatchNeverStartsTheServer(t *testing.T) {
	directory := t.TempDir()
	loadErr := error(nil)
	application := Application[testSettings]{
		Configuration: func([]string, io.Writer) (bool, error) { return false, nil },
		Load:          func() (testSettings, error) { return testSettings{"paths.data": directory}, loadErr },
		MCP:           func(context.Context, testSettings, string) error { t.Fatal("recovery dispatched MCP"); return nil },
		Command: func([]string, io.Reader, io.Writer, string, string, string, bool, string) (bool, error) {
			t.Fatal("recovery dispatched another command")
			return false, nil
		},
		AuthURL: func(testSettings) string { t.Fatal("recovery resolved an authentication URL"); return "" },
		Run:     func(testSettings) int { t.Fatal("recovery started server"); return 0 },
	}
	getenv := func(string) string { return "" }
	if Execute([]string{"owner-access-disable", "extra"}, nil, io.Discard, getenv, nil, application) != 1 {
		t.Fatal("accepted extra recovery argument")
	}
	loadErr = errors.New("invalid settings")
	if Execute([]string{"owner-access-disable"}, nil, io.Discard, getenv, nil, application) != 1 {
		t.Fatal("ignored configuration failure")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected recovery wrote state")
	}
	loadErr = nil
	if Execute([]string{"owner-access-disable"}, nil, io.Discard, getenv, nil, application) != 0 {
		t.Fatal("valid local recovery failed")
	}
	data, err := os.ReadFile(filepath.Join(directory, "owner-access.json"))
	if err != nil || string(data) != `{"enabled":false,"endpoint":"","privateKey":"","devices":[]}` {
		t.Fatalf("recovery state = %q, %v", data, err)
	}
}

func TestPublicGatewayRejectsInvalidInvocationBeforeApplicationLoading(t *testing.T) {
	for _, scenario := range []struct {
		args   []string
		getenv func(string) string
	}{
		{[]string{"public-gateway", "extra"}, func(string) string { return "" }},
		{[]string{"public-gateway"}, nil},
		{[]string{"public-gateway"}, func(string) string { return "invalid host" }},
	} {
		if Execute(scenario.args, nil, io.Discard, scenario.getenv, nil, Application[testSettings]{}) != 1 {
			t.Fatal("invalid gateway invocation succeeded")
		}
	}
}
