package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var transportCache sync.Map // map[string]*http.Transport

type SushiProxy struct {
	router atomic.Value
}

func NewSushiProxy() *SushiProxy {
	return &SushiProxy{}
}

func (proxy *SushiProxy) RegisterRoutes(router chi.Router) {
	router.Handle("/*", proxy.RouteRequest())
	proxy.router.Store(router)
}

func (proxy *SushiProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router := proxy.router.Load()
	if router == nil {
		http.Error(w, "Router not initialized", http.StatusServiceUnavailable)
		return
	}
	router.(chi.Router).ServeHTTP(w, r)
}

// captureResponseWriter is used to capture and store metadata about the HTTP request along the middleware chain
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	body       bytes.Buffer
}

func newCaptureResponseWriter(w http.ResponseWriter) *captureResponseWriter {
	// Default the status code to 200 in case WriteHeader is not called
	return &captureResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (w *captureResponseWriter) WriteHeader(status int) {
	w.statusCode = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.size += size
	// Note: Content-Length is handled automatically by http.ResponseWriter
	return size, err
}

func (w *captureResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// errorCapturingResponseWriter buffers response to allow for retries on failure
type errorCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	body          bytes.Buffer
	headerWritten bool
}

func (w *errorCapturingResponseWriter) WriteHeader(status int) {
	w.statusCode = status
	w.headerWritten = true
}

func (w *errorCapturingResponseWriter) Write(data []byte) (int, error) {
	// If WriteHeader wasn't called, default to 200
	if !w.headerWritten {
		w.statusCode = 200
		w.headerWritten = true
	}
	return w.body.Write(data)
}

func (proxy *SushiProxy) RouteRequest() http.HandlerFunc {
	// Create shared aggregation handler
	aggregationHandler := NewAggregationHandler()

	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("Handing request: " + req.URL.Path)

		captureWriter := newCaptureResponseWriter(w)

		// Start logging start time
		ctx := context.WithValue(req.Context(), constant.CONTEXT_START_TIME, time.Now())
		ctx = context.WithValue(ctx, constant.CONTEXT_CAPTURE_WRITER, captureWriter)
		req = req.WithContext(ctx)

		// TODO: support other content-types
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")

		// Register plugins from global, service and routes using the plugin manager.
		pluginManager, err := NewPluginManagerFromConfig(req)
		if err != nil {
			slog.Info(err.Error())
			err.WriteJSONResponse(w)
			return
		}

		// Add the compulsory response handler plugin...
		pluginManager.RegisterPlugin(NewResponseHandlerPlugin(map[string]interface{}{
			"capture_writer": captureWriter,
		}))

		// Define the final handler that forwards the request to the upstream
		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is an aggregation route (has backends configured)
			_, route, routeErr := util.GetServiceAndRouteFromRequest(GetGlobalProxyConfig(), r)
			if routeErr == nil && IsAggregationRoute(route) {
				// Handle as aggregation request
				slog.Info("Handling aggregation request", "route", route.Name, "backends", len(route.Backends))
				aggregationHandler.HandleAggregation(w, r, route)
				return
			}

			// Otherwise, handle as normal proxy pass
			proxyErr := proxy.HandleProxyPass(w, r)
			if proxyErr != nil {
				slog.Info(proxyErr.Error())
				proxyErr.WriteJSONResponse(w)
			}
		})

		// Execute all plugins as a middleware chain
		chainedHandler := pluginManager.ExecutePlugins(finalHandler)

		// Execute the request (plugins + proxying).
		chainedHandler.ServeHTTP(captureWriter, req)

		// After whole request lifecycle, write the response from the upstream API to the client.
		w.Write(captureWriter.body.Bytes())
	}
}

func (s *SushiProxy) HandleProxyPass(w http.ResponseWriter, req *http.Request) *model.HttpError {
	// 1. Get Service Configuration
	matchedService, _, err := util.GetServiceAndRouteFromRequest(GetGlobalProxyConfig(), req)
	if err != nil {
		return err
	}

	maxRetries := matchedService.RetryOptions.MaxRetries
	retryInterval := matchedService.RetryOptions.RetryIntervalMs
	if retryInterval <= 0 {
		retryInterval = 100 // Default to 100ms
	}

	var lasErr *model.HttpError

	// Create retry handle for tracking failed upstreams (Kong pattern)
	retryHandle := NewRetryHandle(matchedService.Name)

	// Retry Loop
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			retryHandle.IncrementRetry()
			// Exponential Backoff: interval * 2^(attempt-1)
			backoffDuration := time.Duration(retryInterval*int(math.Pow(2, float64(attempt-1)))) * time.Millisecond
			slog.Info(fmt.Sprintf("Retry attempt %d/%d for service %s after %v (excluding %d failed upstreams)",
				attempt, maxRetries, matchedService.Name, backoffDuration, len(retryHandle.FailedUpstreams)))
			time.Sleep(backoffDuration)
		}

		// 2. Select Upstream (Load Balancing) - Re-select on every attempt for failover
		// Use retry-aware selection that excludes previously failed upstreams
		upstreamStartTime := time.Now()
		selectedUpstream, proxyURL, convErr := s.selectUpstreamWithRetry(matchedService, req, retryHandle.GetFailedUpstreams())
		if convErr != nil {
			// If no upstream available, no point retrying immediately unless config changes, but checking config is handled by others.
			// Just return error.
			return convErr
		}

		target, parseErr := url.Parse(proxyURL)
		if parseErr != nil {
			return &model.HttpError{
				Code:     "ERROR_PARSING_PROXY_URL",
				Message:  "Error parsing URL when handling request.",
				HttpCode: http.StatusInternalServerError,
			}
		}

		// Increment active connection count for Least Connections algorithm
		if selectedUpstream != nil {
			IncrementActiveConnections(matchedService.Name, selectedUpstream.Id)
		}

		// 3. Execute Proxy Request with Buffer
		// We need to capture the response to see if it failed
		bufferedWriter := &errorCapturingResponseWriter{ResponseWriter: w}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Setup Transport with mTLS if enabled and OpenTelemetry
		transport, transportErr := s.getTransport(matchedService)
		if transportErr != nil {
			slog.Error("Failed to create transport", "error", transportErr)
			// Decrement before returning
			if selectedUpstream != nil {
				DecrementActiveConnections(matchedService.Name, selectedUpstream.Id)
			}
			return &model.HttpError{
				Code:     "INTERNAL_SERVER_ERROR",
				Message:  "Failed to configure upstream transport",
				HttpCode: http.StatusInternalServerError,
			}
		}
		proxy.Transport = transport

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			// Save original path before Director modifies it (because Director joins target path + req path)
			// If target path is already full (from selectUpstream), this causes duplication/looping.
			savedPath := req.URL.Path

			originalDirector(req)

			// Logic to strip base_path and forward remaining path
			reqPath := savedPath
			if matchedService.BasePath != "" && strings.HasPrefix(reqPath, matchedService.BasePath) {
				reqPath = strings.TrimPrefix(reqPath, matchedService.BasePath)
			}

			// Ensure reqPath starts with /
			if !strings.HasPrefix(reqPath, "/") {
				reqPath = "/" + reqPath
			}

			// Force the path to be what we calculated (ignore what Director did)
			req.URL.Path = reqPath

			// Ensure headers are set (Director does this, but we ensure Host is correct)
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Header.Set(constant.X_FORWARDED_HOST, req.Header.Get("Host"))
			req.Header.Set(constant.X_FORWARDED_FOR, req.RemoteAddr)
			req.Host = target.Host
		}

		// Check for failures using ErrorHandler
		var proxyError bool
		proxy.ErrorHandler = func(rw http.ResponseWriter, r *http.Request, e error) {
			slog.Error("Proxy error", "error", e)
			proxyError = true
			// We don't write to rw yet, we mark as error
		}

		proxy.ServeHTTP(bufferedWriter, req)

		// Decrement active connection count
		if selectedUpstream != nil {
			DecrementActiveConnections(matchedService.Name, selectedUpstream.Id)
		}

		// Determine if successful
		// Fail if:
		// 1. Proxy Error Handler was called (network error)
		// 2. Status code is 5xx (Server Error)
		// 3. Status code is 502/503/504

		isSuccess := !proxyError && bufferedWriter.statusCode < 500

		// Record passive health check result
		if selectedUpstream != nil {
			GlobalHealthChecker.RecordPassiveResult(matchedService, selectedUpstream.Id, isSuccess)
		}

		// Record upstream metrics
		upstreamDuration := time.Since(upstreamStartTime)
		RecordUpstreamRequest(matchedService.Name, target.Host, bufferedWriter.statusCode, upstreamDuration.Seconds())
		RecordConsumerRequest(req.Context(), matchedService.Name, statusCodeToLabel(bufferedWriter.statusCode))

		// Record latency for EWMA-based load balancing (only on success to avoid skewing with timeout values)
		if selectedUpstream != nil && isSuccess {
			RecordLatency(matchedService.Name, selectedUpstream.Id, upstreamDuration)
		}

		if isSuccess {
			// Success! Write buffered response to actual writer
			w.WriteHeader(bufferedWriter.statusCode)
			w.Write(bufferedWriter.body.Bytes())
			return nil
		}

		// Failure case - mark this upstream as failed for retry tracking
		if selectedUpstream != nil {
			retryHandle.MarkFailed(selectedUpstream.Id)
			slog.Debug("Marked upstream as failed", "upstreamId", selectedUpstream.Id)
		}

		lasErr = &model.HttpError{
			Code:     "UPSTREAM_ERROR",
			Message:  fmt.Sprintf("Upstream failed with status %d", bufferedWriter.statusCode),
			HttpCode: http.StatusBadGateway,
		}

		// If retries exhausted, don't continue loop
		if attempt == maxRetries {
			break
		}
	}

	// If we got here, all retries failed.
	// Write the last error or the buffered 5xx response?
	// The bufferedWriter from the last attempt has the authentic upstream error page.
	// But our HandleProxyPass signature returns *model.HttpError which the caller converts to JSON.
	// The caller (RouteRequest) calls err.WriteJSONResponse(w).
	return lasErr
}

// getTransport creates a transport with optimized connection pooling and respects service-specific timeouts
func (s *SushiProxy) getTransport(service *model.Service) (http.RoundTripper, error) {
	// Create a cache key based on timeouts and TLS config
	cacheKey := fmt.Sprintf("%d-%d-%d-%v-%s-%s-%s",
		service.ConnectTimeout, service.ReadTimeout, service.WriteTimeout,
		service.TLS.Enabled, service.TLS.CaCertPath, service.TLS.CertPath, service.TLS.KeyPath)

	if val, ok := transportCache.Load(cacheKey); ok {
		return val.(http.RoundTripper), nil
	}

	// Create optimized transport
	connectTimeout := time.Duration(service.ConnectTimeout) * time.Millisecond
	if connectTimeout == 0 {
		connectTimeout = 5 * time.Second // Default
	}

	readTimeout := time.Duration(service.ReadTimeout) * time.Millisecond
	if readTimeout == 0 {
		readTimeout = 60 * time.Second // Default
	}

	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: readTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if service.TLS.Enabled {
		tlsConfig := &tls.Config{} // #nosec G402
		if service.TLS.CaCertPath != "" {
			caCert, err := os.ReadFile(service.TLS.CaCertPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}
		if service.TLS.CertPath != "" && service.TLS.KeyPath != "" {
			cert, err := tls.LoadX509KeyPair(service.TLS.CertPath, service.TLS.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load cert/key: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		baseTransport.TLSClientConfig = tlsConfig
	}

	var finalTransport http.RoundTripper = baseTransport
	if EnableOTEL {
		finalTransport = otelhttp.NewTransport(baseTransport)
	}

	transportCache.Store(cacheKey, finalTransport)
	return finalTransport, nil
}

// Helper to select upstream and construct URL (without retry tracking)
func (s *SushiProxy) selectUpstream(matchedService *model.Service, req *http.Request) (*model.UpstreamTarget, string, *model.HttpError) {
	return s.selectUpstreamWithRetry(matchedService, req, nil)
}

// selectUpstreamWithRetry selects upstream while excluding failed ones
// This implements Kong's failedAddresses pattern for intelligent retry routing
func (s *SushiProxy) selectUpstreamWithRetry(matchedService *model.Service, req *http.Request, failedUpstreams map[string]bool) (*model.UpstreamTarget, string, *model.HttpError) {
	// 0. Service Discovery
	if matchedService.ServiceName != "" {
		// If DiscoveryProvider is set (or just default/any), look it up
		if container.Global != nil && container.Global.Registry != nil {
			resolvedURL, err := container.Global.Registry.GetServiceURL(matchedService.ServiceName)
			if err != nil {
				slog.Error("Service discovery failed", "service", matchedService.ServiceName, "error", err)
				return nil, "", &model.HttpError{
					Code:     "SERVICE_DISCOVERY_ERROR",
					Message:  fmt.Sprintf("Failed to resolve service %s: %v", matchedService.ServiceName, err),
					HttpCode: http.StatusBadGateway,
				}
			}
			return nil, resolvedURL, nil
		} else {
			slog.Warn("Service Name configured but no Registry initialized", "service", matchedService.ServiceName)
		}
	}

	// 0.1 Direct URL (Kong style)
	if matchedService.URL != "" {
		return nil, matchedService.URL, nil
	}

	loadBalancer := NewLoadBalancer(GlobalHealthChecker)

	// Get client IP for load balancing algorithms that need it
	clientIP, _ := util.GetHostIp(req.RemoteAddr)

	// Select upstream, potentially excluding failed ones for retry tracking
	var upstreamIndex int
	if len(failedUpstreams) > 0 {
		// Use retry-aware selection that excludes failed upstreams
		upstreamIndex = loadBalancer.GetNextUpstreamWithRetry(*matchedService, clientIP, failedUpstreams)
	} else {
		// Get upstream config for hash_on settings (used by consistent-hashing)
		upstreamConfig := GetUpstreamConfigForService(GetGlobalProxyConfig(), matchedService)
		// Use the enhanced load balancer that extracts hash values based on upstream config
		upstreamIndex = loadBalancer.GetNextUpstreamWithRequest(*matchedService, req, upstreamConfig)
	}

	if upstreamIndex == model.NoUpstreamsAvailable {
		return nil, "", &model.HttpError{
			Code:     "ERROR_NO_UPSTREAMS_AVAILABLE",
			Message:  "No upstreams available for service: " + matchedService.Name,
			HttpCode: http.StatusServiceUnavailable,
		}
	}

	upstream := matchedService.Upstreams[upstreamIndex]

	// 2. Select the specific route that matched this request to determine strip_path behavior
	_, matchedRoute, _ := util.GetServiceAndRouteFromRequest(GetGlobalProxyConfig(), req)

	path := req.URL.Path
	if matchedRoute != nil {
		stripPath := true // Default to true if not specified
		if matchedRoute.StripPath != nil {
			stripPath = *matchedRoute.StripPath
		}

		if stripPath {
			path = s.stripMatchedPath(req.URL.Path, matchedRoute)
		}
	}

	proxyURL := fmt.Sprintf("%s://%s%s", matchedService.Protocol, upstream.Target, path)
	return &upstream, proxyURL, nil
}

// stripMatchedPath removes the matched prefix from the request path
func (s *SushiProxy) stripMatchedPath(path string, route *model.Route) string {
	// Find which path in the route matched
	for _, p := range route.Paths {
		if strings.HasPrefix(path, p) {
			stripped := strings.TrimPrefix(path, p)
			if !strings.HasPrefix(stripped, "/") {
				stripped = "/" + stripped
			}
			return stripped
		}
	}
	// Fallback for deprecated single path
	if route.Path != "" && strings.HasPrefix(path, route.Path) {
		stripped := strings.TrimPrefix(path, route.Path)
		if !strings.HasPrefix(stripped, "/") {
			stripped = "/" + stripped
		}
		return stripped
	}
	return path
}
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
