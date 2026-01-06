package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// Shadow Traffic Plugin - Duplicates requests to test backend
// Use Cases:
// - Test new backend version with production traffic
// - Validate changes before full deployment
// - Compare old vs new backend behavior

var (
	// Shadow traffic metrics
	ShadowRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_shadow_requests_total",
			Help: "Total number of shadow requests sent",
		},
		[]string{"target", "route"},
	)

	ShadowErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_shadow_errors_total",
			Help: "Total number of shadow request errors",
		},
		[]string{"target", "route", "error_type"},
	)

	ShadowDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sushi_gateway_shadow_duration_seconds",
			Help:    "Shadow request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"target", "route"},
	)
)

type ShadowTrafficPlugin struct {
	config     map[string]interface{}
	TargetURL  string
	SampleRate float64 // 0.0 to 1.0 (0.1 = 10% of traffic)
	Async      bool    // Fire and forget (true) or wait for response (false)
	Timeout    int     // Timeout in milliseconds (default: 5000)
	client     *http.Client
	counter    uint64 // For sampling
}

func NewShadowTrafficPlugin(config map[string]interface{}) *Plugin {
	plugin := &ShadowTrafficPlugin{
		config:     config,
		SampleRate: 1.0, // Default: shadow 100% of traffic
		Async:      true,
		Timeout:    5000,
	}

	if targetURL, ok := config["target_url"].(string); ok {
		plugin.TargetURL = targetURL
	}

	if sampleRate, ok := config["sample_rate"].(float64); ok {
		plugin.SampleRate = sampleRate
	}

	if async, ok := config["async"].(bool); ok {
		plugin.Async = async
	}

	if timeout, ok := config["timeout"].(float64); ok {
		plugin.Timeout = int(timeout)
	}

	// Create HTTP client with timeout
	plugin.client = &http.Client{
		Timeout: time.Duration(plugin.Timeout) * time.Millisecond,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &Plugin{
		Name:      constant.PLUGIN_SHADOW_TRAFFIC,
		Priority:  900, // Run late (after auth)
		Handler:   plugin,
		Validator: plugin,
	}
}

func (p *ShadowTrafficPlugin) Validate() error {
	if p.TargetURL == "" {
		return errors.New("target_url is required for shadow_traffic plugin")
	}
	return nil
}

func (p *ShadowTrafficPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sample traffic based on sample_rate
		if p.shouldShadow() {
			// Read request body (need to buffer for duplication)
			var bodyBytes []byte
			if r.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					slog.Warn("Shadow traffic: failed to read request body", "error", err)
				} else {
					// Restore body for main request
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					if p.Async {
						// Fire and forget - don't block main request
						go p.sendShadowRequest(r, bodyBytes)
					} else {
						// Synchronous - wait for response (for testing)
						p.sendShadowRequest(r, bodyBytes)
					}
				}
			}
		}

		// Continue with main request
		next.ServeHTTP(w, r)
	})
}

func (p *ShadowTrafficPlugin) shouldShadow() bool {
	if p.SampleRate >= 1.0 {
		return true
	}
	if p.SampleRate <= 0.0 {
		return false
	}

	// Simple counter-based sampling
	count := atomic.AddUint64(&p.counter, 1)
	threshold := uint64(1.0 / p.SampleRate)
	return count%threshold == 0
}

func (p *ShadowTrafficPlugin) sendShadowRequest(originalReq *http.Request, bodyBytes []byte) {
	startTime := time.Now()
	routeName := originalReq.URL.Path

	defer func() {
		duration := time.Since(startTime).Seconds()
		ShadowDuration.WithLabelValues(p.TargetURL, routeName).Observe(duration)
	}()

	// Build shadow request
	shadowURL := p.TargetURL + originalReq.URL.Path
	if originalReq.URL.RawQuery != "" {
		shadowURL += "?" + originalReq.URL.RawQuery
	}

	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	shadowReq, err := http.NewRequestWithContext(
		context.Background(),
		originalReq.Method,
		shadowURL,
		bodyReader,
	)
	if err != nil {
		slog.Warn("Shadow traffic: failed to create request",
			"target", p.TargetURL,
			"error", err)
		ShadowErrorsTotal.WithLabelValues(p.TargetURL, routeName, "create_request").Inc()
		return
	}

	// Copy headers from original request
	for key, values := range originalReq.Header {
		for _, value := range values {
			shadowReq.Header.Add(key, value)
		}
	}

	// Add marker header to identify shadow traffic
	shadowReq.Header.Set("X-Shadow-Request", "true")
	shadowReq.Header.Set("X-Shadow-Source", "sushi-gateway")

	// Send request
	resp, err := p.client.Do(shadowReq)
	if err != nil {
		slog.Debug("Shadow traffic: request failed",
			"target", p.TargetURL,
			"method", shadowReq.Method,
			"path", shadowReq.URL.Path,
			"error", err)
		ShadowErrorsTotal.WithLabelValues(p.TargetURL, routeName, "network").Inc()
		return
	}
	defer resp.Body.Close()

	// Record success
	ShadowRequestsTotal.WithLabelValues(p.TargetURL, routeName).Inc()

	// Log response status for debugging
	slog.Debug("Shadow traffic: request completed",
		"target", p.TargetURL,
		"method", shadowReq.Method,
		"path", shadowReq.URL.Path,
		"status", resp.StatusCode,
		"duration_ms", time.Since(startTime).Milliseconds())

	// Drain response body to reuse connection
	_, _ = io.Copy(io.Discard, resp.Body)
}
