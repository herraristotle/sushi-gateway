package gateway

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// Prometheus metrics for Sushi Gateway
var (
	EnablePrometheus         = false
	EnablePerConsumerMetrics = false

	// Request metrics
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"service", "route", "method", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sushi_gateway_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "route", "method"},
	)

	RequestsPerConsumer = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_requests_per_consumer_total",
			Help: "Total number of HTTP requests processed per consumer",
		},
		[]string{"consumer", "service", "status"},
	)

	// Rate limiting metrics
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_rate_limit_hits_total",
			Help: "Total number of requests that hit rate limit",
		},
		[]string{"scope", "window"},
	)

	RateLimitAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_rate_limit_allowed_total",
			Help: "Total number of requests allowed by rate limiter",
		},
		[]string{"scope"},
	)

	// Cache metrics
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sushi_gateway_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sushi_gateway_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	CacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sushi_gateway_cache_entries",
			Help: "Current number of entries in cache (approximate)",
		},
	)

	// Circuit breaker metrics
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sushi_gateway_circuit_breaker_state",
			Help: "Current circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"service"},
	)

	CircuitBreakerTrips = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_circuit_breaker_trips_total",
			Help: "Total number of times circuit breaker tripped to open",
		},
		[]string{"service"},
	)

	// Upstream metrics
	UpstreamRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_upstream_requests_total",
			Help: "Total number of requests to upstream services",
		},
		[]string{"service", "upstream", "status"},
	)

	UpstreamDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sushi_gateway_upstream_duration_seconds",
			Help:    "Upstream request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "upstream"},
	)

	UpstreamHealthy = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sushi_gateway_upstream_healthy",
			Help: "Whether upstream is healthy (1=healthy, 0=unhealthy)",
		},
		[]string{"service", "upstream"},
	)

	// Aggregation metrics
	AggregationRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_aggregation_requests_total",
			Help: "Total number of aggregation (BFF) requests",
		},
		[]string{"route"},
	)

	AggregationBackendSuccess = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_aggregation_backend_success_total",
			Help: "Total successful backend calls in aggregation",
		},
		[]string{"route", "backend"},
	)

	AggregationBackendFailure = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_aggregation_backend_failure_total",
			Help: "Total failed backend calls in aggregation",
		},
		[]string{"route", "backend"},
	)

	// Redis metrics
	RedisOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"operation", "status"},
	)

	RedisLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sushi_gateway_redis_latency_seconds",
			Help:    "Redis operation latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
		[]string{"operation"},
	)

	// Active connections
	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sushi_gateway_active_connections",
			Help: "Number of currently active connections",
		},
	)

	// Plugin execution metrics
	PluginExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sushi_gateway_plugin_execution_seconds",
			Help:    "Plugin execution time in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		},
		[]string{"plugin"},
	)
)

// RecordRequest records a completed request with its metrics
func RecordRequest(service, route, method string, statusCode int, durationSeconds float64) {
	if !EnablePrometheus {
		return
	}
	status := statusCodeToLabel(statusCode)
	RequestsTotal.WithLabelValues(service, route, method, status).Inc()
	RequestDuration.WithLabelValues(service, route, method).Observe(durationSeconds)
}

// RecordConsumerRequest records metrics with consumer context
func RecordConsumerRequest(ctx context.Context, service, status string) {
	if !EnablePrometheus || !EnablePerConsumerMetrics {
		return
	}
	consumerId, ok := ctx.Value(constant.CONTEXT_CONSUMER_ID).(string)
	if !ok || consumerId == "" {
		consumerId = "anonymous"
	}
	RequestsPerConsumer.WithLabelValues(consumerId, service, status).Inc()
}

// RecordRateLimitHit records when a request is rate limited
func RecordRateLimitHit(scope, window string) {
	if !EnablePrometheus {
		return
	}
	RateLimitHits.WithLabelValues(scope, window).Inc()
}

// RecordRateLimitAllowed records when a request passes rate limiting
func RecordRateLimitAllowed(scope string) {
	if !EnablePrometheus {
		return
	}
	RateLimitAllowed.WithLabelValues(scope).Inc()
}

// RecordCacheHit records a cache hit
func RecordCacheHit() {
	if !EnablePrometheus {
		return
	}
	CacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss() {
	if !EnablePrometheus {
		return
	}
	CacheMisses.Inc()
}

// RecordCircuitBreakerState records the current circuit breaker state
func RecordCircuitBreakerState(service string, state int) {
	if !EnablePrometheus {
		return
	}
	CircuitBreakerState.WithLabelValues(service).Set(float64(state))
}

// RecordCircuitBreakerTrip records when a circuit breaker trips
func RecordCircuitBreakerTrip(service string) {
	if !EnablePrometheus {
		return
	}
	CircuitBreakerTrips.WithLabelValues(service).Inc()
}

// RecordUpstreamRequest records an upstream request
func RecordUpstreamRequest(service, upstream string, statusCode int, durationSeconds float64) {
	if !EnablePrometheus {
		return
	}
	status := statusCodeToLabel(statusCode)
	UpstreamRequestsTotal.WithLabelValues(service, upstream, status).Inc()
	UpstreamDuration.WithLabelValues(service, upstream).Observe(durationSeconds)
}

// RecordUpstreamHealth records upstream health status
func RecordUpstreamHealth(service, upstream string, healthy bool) {
	if !EnablePrometheus {
		return
	}
	val := 0.0
	if healthy {
		val = 1.0
	}
	UpstreamHealthy.WithLabelValues(service, upstream).Set(val)
}

// RecordAggregationRequest records an aggregation request
func RecordAggregationRequest(route string) {
	if !EnablePrometheus {
		return
	}
	AggregationRequestsTotal.WithLabelValues(route).Inc()
}

// RecordAggregationBackend records backend result in aggregation
func RecordAggregationBackend(route, backend string, success bool) {
	if !EnablePrometheus {
		return
	}
	if success {
		AggregationBackendSuccess.WithLabelValues(route, backend).Inc()
	} else {
		AggregationBackendFailure.WithLabelValues(route, backend).Inc()
	}
}

// RecordRedisOperation records a Redis operation
func RecordRedisOperation(operation string, success bool, durationSeconds float64) {
	if !EnablePrometheus {
		return
	}
	status := "success"
	if !success {
		status = "error"
	}
	RedisOperations.WithLabelValues(operation, status).Inc()
	RedisLatency.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordPluginExecution records plugin execution time
func RecordPluginExecution(pluginName string, durationSeconds float64) {
	if !EnablePrometheus {
		return
	}
	PluginExecutionDuration.WithLabelValues(pluginName).Observe(durationSeconds)
}

// Helper to convert status code to label category
func statusCodeToLabel(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

func EnableMetrics(config map[string]interface{}) {
	EnablePrometheus = true
	if val, ok := config["per_consumer"].(bool); ok {
		EnablePerConsumerMetrics = val
	}
}
