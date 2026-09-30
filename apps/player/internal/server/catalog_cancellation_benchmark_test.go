package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func BenchmarkNativeCatalogCancelledMetadataSearch(b *testing.B) {
	for _, condition := range []string{"already-cancelled", "10ms-deadline"} {
		b.Run(condition, func(b *testing.B) { benchmarkCancelledMetadataSearch(b, condition) })
	}
}

func benchmarkCancelledMetadataSearch(b *testing.B, condition string) {
	handler, baseRequest := cancelledMetadataSearchFixture()
	cancelled, cancel := context.WithCancel(baseRequest.Context())
	cancel()
	cancelledRequest := baseRequest.WithContext(cancelled)
	responseBytes, status := 0, 0
	b.ReportAllocs()
	for b.Loop() {
		request := cancelledRequest
		var timeoutCancel context.CancelFunc
		if condition == "10ms-deadline" {
			ctx, cleanup := context.WithTimeout(baseRequest.Context(), 10*time.Millisecond)
			request, timeoutCancel = baseRequest.WithContext(ctx), cleanup
		}
		response := httptest.NewRecorder()
		handler(response, request)
		ended := request.Context().Err()
		if timeoutCancel != nil {
			timeoutCancel()
		}
		if ended == nil {
			b.Fatal("fixture did not exercise cancellation")
		}
		verifyCancelledBenchmarkResponse(b, response)
		responseBytes, status = response.Body.Len(), response.Code
	}
	b.ReportMetric(float64(responseBytes), "response-bytes")
	b.ReportMetric(float64(status), "status-code")
}

func verifyCancelledBenchmarkResponse(b *testing.B, response *httptest.ResponseRecorder) {
	b.Helper()
	switch response.Code {
	case http.StatusOK:
		if !strings.Contains(response.Body.String(), `"id":"9999"`) {
			b.Fatal("baseline omitted known title")
		}
	case http.StatusServiceUnavailable:
		if strings.Contains(response.Body.String(), `"items"`) {
			b.Fatal("cancelled search returned a partial page")
		}
	default:
		b.Fatal(response.Code)
	}
}
