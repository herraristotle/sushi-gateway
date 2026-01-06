package gateway

import (
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func BenchmarkRouter_Routing(b *testing.B) {

	counts := []int{10, 100, 1000, 5000}

	for _, n := range counts {
		b.Run(fmt.Sprintf("Routes_%d", n), func(b *testing.B) {
			proxy := NewSushiProxy()
			config := &model.ProxyConfig{
				Services: []model.Service{{
					Name: "bench-service",
				}},
			}

			// Generate N routes
			routes := make([]model.Route, n)
			for i := 0; i < n; i++ {
				routes[i] = model.Route{
					Name:  fmt.Sprintf("route-%d", i),
					Paths: []string{fmt.Sprintf("/api/v1/%d", i)},
				}
			}
			config.Services[0].Routes = routes
			proxy.UpdateRouter(config)

			// Request targeting the LAST route (worst case for linear scan)
			targetPath := fmt.Sprintf("/api/v1/%d", n-1)
			req := httptest.NewRequest("GET", targetPath, nil)
			// Reset recorder is expensive to create inside loop?
			// ServeHTTP writes to w.
			// Ideally we reuse w?
			// httptest.Recorder buffers body. Infinite growth?
			// Better is to use a specific optimized mock writer if possible,
			// or just NewRecorder inside. Allocations might dominate.
			// But allocs are constant per request.

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Create new recorder to simulate fresh response writer
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)
			}
		})
	}
}
