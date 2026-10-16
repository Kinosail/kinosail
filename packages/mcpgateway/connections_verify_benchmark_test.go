package mcpgateway

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func BenchmarkVerifyTokenParallel(b *testing.B) {
	now := time.Now()
	profile := Principal{ID: "viewer", Name: "Viewer"}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{profile.ID: profile}}, &memoryState{})
	for index := range mcpConnectionLimit {
		id := strconv.Itoa(index)
		connections.grants[id] = mcpOAuthGrant{ID: id, ClientID: "agent", ProfileID: profile.ID, Scopes: []string{ReadScope}, AccessHash: secretHash("access-" + id), AccessExpires: now.Add(time.Hour).Unix(), RefreshHash: secretHash("refresh-" + id), RefreshExpires: now.Add(time.Hour).Unix()}
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := connections.VerifyToken(b.Context(), "access-127", httptest.NewRequestWithContext(b.Context(), http.MethodPost, "/mcp", nil)); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
