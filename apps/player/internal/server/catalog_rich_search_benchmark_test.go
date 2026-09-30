package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkNativeCatalogRichSearch(b *testing.B) {
	for _, scenario := range []struct {
		name, target string
		total        int
	}{
		{"title-all", "/api/v1/library?view=movies&q=Movie", 10000},
		{"title-exact", "/api/v1/library?view=movies&q=Movie+9999", 1},
		{"metadata-all", "/api/v1/library?view=movies&q=quiet+journey", 10000},
		{"absent", "/api/v1/library?view=movies&q=Absent+owl", 0},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			handler, _ := cancelledMetadataSearchFixture()
			request := ownerRequest(scenario.target)
			ctx, cancel := context.WithCancel(request.Context())
			b.Cleanup(cancel)
			request = request.WithContext(ctx)
			want := []byte(fmt.Sprintf(`"total":%d,`, scenario.total))
			responseBytes := 0
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), want) {
					b.Fatalf("wrong search result status=%d bytes=%d", response.Code, response.Body.Len())
				}
				responseBytes = response.Body.Len()
			}
			b.ReportMetric(float64(responseBytes), "response-bytes")
		})
	}
}
