package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

type mockRoundTripper struct {
	statusCode int
	err        error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	resp := httptest.NewRecorder().Result()
	resp.StatusCode = m.statusCode
	return resp, nil
}

func TestCircuitBreakerTransport(t *testing.T) {
	target := "test-target-1"
	mockService := &model.Service{Name: "test-service"}
	upstreamID := "u1"
	cb := GlobalTargetCBManager.GetBreaker(target, mockService, upstreamID)

	// Reset state if needed (sony/gobreaker doesn't have easy reset, but we can just use a unique name)
	target = fmt.Sprintf("test-target-%d", time.Now().UnixNano())
	cb = GlobalTargetCBManager.GetBreaker(target, mockService, upstreamID)

	base := &mockRoundTripper{statusCode: 500}
	transport := &CircuitBreakerTransport{
		Target:     target,
		Service:    mockService,
		UpstreamID: upstreamID,
		Base:       base,
	}

	// Fail 10 times to trip the breaker (based on our default settings in TargetCBManager)
	for i := 0; i < 10; i++ {
		resp, err := transport.RoundTrip(httptest.NewRequest("GET", "/", nil))
		assert.Error(t, err)
		assert.Nil(t, resp)
	}

	// Now it should be open
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Next request should fail with ErrOpenState before even calling the base
	resp, err := transport.RoundTrip(httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, gobreaker.ErrOpenState, err)
	assert.Nil(t, resp)
}

func TestTargetCBManager_Allow(t *testing.T) {
	target := fmt.Sprintf("test-allow-%d", time.Now().UnixNano())
	mockService := &model.Service{Name: "test-service-allow"}
	upstreamID := "u2"

	assert.True(t, GlobalTargetCBManager.Allow(target, mockService, upstreamID))

	cb := GlobalTargetCBManager.GetBreaker(target, mockService, upstreamID)

	// Manually trip it if we could, but we'll use the transport
	base := &mockRoundTripper{statusCode: 500}
	transport := &CircuitBreakerTransport{
		Target:     target,
		Service:    mockService,
		UpstreamID: upstreamID,
		Base:       base,
	}

	for i := 0; i < 10; i++ {
		_, _ = transport.RoundTrip(httptest.NewRequest("GET", "/", nil))
	}

	assert.False(t, GlobalTargetCBManager.Allow(target, mockService, upstreamID))
	assert.Equal(t, gobreaker.StateOpen, cb.State())
}
