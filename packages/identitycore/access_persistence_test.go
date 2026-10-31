package identitycore

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestAccessPersistenceRetriesFailedWritesAndLogsRecovery(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	failing, attempts := true, 0
	persistence := &AccessPersistence{}
	persist := func(string, any) error {
		attempts++
		if failing {
			return errors.New("disk full")
		}
		return nil
	}
	const now = 1_800_000_000
	persistence.Save(context.Background(), persist, "request-1", "session-expire", "sessions.json", map[string]Session{}, now)
	if due := persistence.Due(now+AccessRetrySeconds-1, "sessions.json", "api_keys.json", false, false); len(due) != 0 {
		t.Fatalf("retry due too early: %v", due)
	}
	due := persistence.Due(now+AccessRetrySeconds, "sessions.json", "api_keys.json", false, true)
	if due["sessions.json"] != "retry" || due["api_keys.json"] != "api-key-use" {
		t.Fatalf("due writes = %v", due)
	}
	failing = false
	persistence.Save(context.Background(), persist, "", "retry", "sessions.json", map[string]Session{}, now+AccessRetrySeconds)
	if due := persistence.Due(now+10*AccessRetrySeconds, "sessions.json", "api_keys.json", true, false); due["sessions.json"] != "session-use" || len(due) != 1 {
		t.Fatalf("due writes after recovery = %v", due)
	}
	output := logs.String()
	if attempts != 2 || !strings.Contains(output, `"level":"WARN","msg":"Access state write failed"`) || !strings.Contains(output, `"outcome":"retry-scheduled"`) || !strings.Contains(output, `"msg":"Access state write recovered"`) || !strings.Contains(output, `"request_id":"request-1"`) {
		t.Fatalf("attempts=%d logs=%s", attempts, output)
	}
}
