package gateway

import (
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsTransport_RoundTrip(t *testing.T) {
	// Enable metrics
	EnablePrometheus = true

	// Clear metrics for test
	PoolActiveConnections.Reset()

	// Create a mock transport that verifies the gauge is incremented during the call
	baseTransport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			// Verify gauge is 1 for this specific label set
			val := testutil.ToFloat64(PoolActiveConnections.WithLabelValues("test-service", "test-upstream"))
			assert.Equal(t, 1.0, val, "Active connections should be 1 during RoundTrip")
			return &http.Response{StatusCode: 200}, nil
		},
	}

	transport := &MetricsTransport{
		Base:     baseTransport,
		Service:  "test-service",
		Upstream: "test-upstream",
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	transport.RoundTrip(req)

	// Verify gauge is back to 0
	val := testutil.ToFloat64(PoolActiveConnections.WithLabelValues("test-service", "test-upstream"))
	assert.Equal(t, 0.0, val, "Active connections should be 0 after RoundTrip")
}

func TestRecordConfigReload(t *testing.T) {
	EnablePrometheus = true
	ConfigReloadTotal.Reset()

	RecordConfigReload(true, 0.5)

	// Verify success counter
	val := testutil.ToFloat64(ConfigReloadTotal.WithLabelValues("success"))
	assert.Equal(t, 1.0, val)

	RecordConfigReload(false, 0.1)
	val = testutil.ToFloat64(ConfigReloadTotal.WithLabelValues("failure"))
	assert.Equal(t, 1.0, val)
}

type mockTransport struct {
	roundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}
