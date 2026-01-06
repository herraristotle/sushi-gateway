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
	"path/filepath"
	"regexp"
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

var GlobalSushiProxy *SushiProxy

type SushiProxy struct {
	router atomic.Value
}

func NewSushiProxy() *SushiProxy {
	proxy := &SushiProxy{}
	if GlobalSushiProxy == nil {
		GlobalSushiProxy = proxy
	}
	return proxy
}

func (proxy *SushiProxy) UpdateRouter(config *model.ProxyConfig) {
	router := BuildRouterFromConfig(config, proxy)
	proxy.router.Store(router)
	slog.Info("Router updated successfully")
}

// BuildRouterFromConfig creates a new chi router based on the provided configuration
func BuildRouterFromConfig(config *model.ProxyConfig, proxy *SushiProxy) *chi.Mux {
	router := chi.NewRouter()

	// Register /readyz on the internal router as well just in case, though main.go handles it on the parent.
	// Actually main.go handles /readyz on the parent router before Proxy.ServeHTTP is called.
	// But if we want to support it here:
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	for _, svc := range config.Services {
		svc := svc // capture loop var
		for _, rt := range svc.Routes {
			rt := rt // capture loop var

			// Pre-build handler chain for this route
			handler := proxy.createHandlerForRoute(&svc, &rt)

			for _, path := range rt.Paths {
				// Handle prefix matching for Chi
				// If path is "/api", we want to match "/api", "/api/" and "/api/*"
				// Chi handles specific paths.
				registerPath := path
				if !strings.HasSuffix(registerPath, "*") {
					if strings.HasSuffix(registerPath, "/") {
						registerPath += "*"
					} else {
						registerPath += "/*"
						// Also register exact match
						router.Handle(path, handler)
					}
				}
				router.Handle(registerPath, handler)
			}
			// Legacy path support
			if rt.Path != "" {
				router.Handle(rt.Path+"/*", handler)
				router.Handle(rt.Path, handler)
			}
		}
	}
	return router
}

// createHandlerForRoute builds the middleware chain for a specific route
func (proxy *SushiProxy) createHandlerForRoute(service *model.Service, route *model.Route) http.HandlerFunc {
	// 1. Create Aggregation Handler shared instance
	aggregationHandler := NewAggregationHandler()

	// 2. Build Plugin Manager with specific plugins
	pm := NewPluginManager()

	// Load Global Plugins
	globalConfig := GetGlobalProxyConfig()
	if globalConfig != nil {
		for _, pc := range globalConfig.Plugins {
			pm.loadConfig(pc)
		}
	}

	// Load Route Plugins (Replicating existing behavior: Route before Service)
	for _, pc := range route.Plugins {
		pm.loadConfig(pc)
	}

	// Load Service Plugins (Replicating existing behavior: Service last overwrites Route?!)
	for _, pc := range service.Plugins {
		pm.loadConfig(pc)
	}

	// 3. Define Final Handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No need to lookup Service/Route! We have them in closure.

		if IsAggregationRoute(route) {
			slog.Info("Handling aggregation request", "route", route.Name, "backends", len(route.Backends))
			aggregationHandler.HandleAggregation(w, r, route)
			return
		}

		proxyErr := proxy.HandleProxyPass(w, r, service, route)
		if proxyErr != nil {
			slog.Info(proxyErr.Error())
			proxyErr.WriteJSONResponse(w)
		}
	})

	// 4. Wrap with Plugins
	// Register the compulsory response handler plugin...
	pm.RegisterPlugin(NewResponseHandlerPlugin(nil))

	chainedHandler := pm.ExecutePlugins(finalHandler)

	// 5. Wrap with Compulsory Logic (CaptureWriter, Logging)
	// We need to construct the outer handler that sets up CaptureWriter and Context
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("Handling request: " + req.URL.Path)

		// Security: Validate request framing
		if err := validateRequestFraming(req); err != nil {
			err.WriteJSONResponse(w)
			return
		}

		captureWriter := newCaptureResponseWriter(w)

		// Start logging start time
		ctx := context.WithValue(req.Context(), constant.CONTEXT_START_TIME, time.Now())
		ctx = context.WithValue(ctx, constant.CONTEXT_CAPTURE_WRITER, captureWriter)
		// Inject Matched Service/Route for optimization
		ctx = context.WithValue(ctx, constant.CONTEXT_MATCHED_SERVICE, service)
		ctx = context.WithValue(ctx, constant.CONTEXT_MATCHED_ROUTE, route)

		req = req.WithContext(ctx)

		w.Header().Add("Content-Type", "application/json; charset=UTF-8")

		// Register compulsory ResponseHandlerPlugin dynamically?
		// The original code did: pm.RegisterPlugin(NewResponseHandlerPlugin(...))
		// But here 'pm' is reused across requests (built once).
		// ResponseHandlerPlugin needs 'captureWriter' which is per-request!
		// PROBLEM: Plugins are instantiated ONCE per route.
		// But ResponseHandlerPlugin depends on 'captureWriter' which is created PER REQUEST.
		// Solution: ResponseHandlerPlugin logic should be wrapped manually or it should fetch captureWriter from Context.
		// Let's look at NewResponseHandlerPlugin.

		// If we cannot reuse PM, we cannot pre-build the chain if plugins are stateful per-request.
		// Most plugins are stateless config-based (RateLimit, Auth).
		// ResponseHandlerPlugin is the exception.
		// Let's implement ResponseHandler logic directly here or use a Context-aware plugin.

		// We can execute the static chain, then manually execute ResponseHandler logic/wrapper?
		// Existing ResponseHandlerPlugin seems to just write logs/metrics after next.ServeHTTP?
		// Or it modifies response?

		// If I cannot pre-build the chain fully because of one plugin...
		// I can pre-build the STATIC/CONFIG plugins.
		// And wrap that static chain with the per-request wrappers.

		// The original code:
		// pm.RegisterPlugin(NewResponseHandlerPlugin(map[...]{ "capture_writer": captureWriter }))
		// chainedHandler := pm.ExecutePlugins(finalHandler)
		// chainedHandler.ServeHTTP(...)

		// ResponseHandlerPlugin is critical.
		// Since we want to avoid recreating PM every request...
		// We should handle Response Logic in this outer wrapper function!

		// Execute static plugins
		chainedHandler.ServeHTTP(captureWriter, req)

		// Perform ResponseHandler logic here (post-processing)
		// ... (Logging, Metrics) ...
		// Actually ResponseHandlerPlugin might be doing things BEFORE writing?

		// Let's verify ResponseHandlerPlugin usage.
		// It is just added to the chain.
		// If it's a plugin, it implements `Evaluate` or `Execute`.
		// If it's `Execute(http.Handler) http.Handler`, it wraps.

		// I will modify the ResponseHandlerPlugin to accept captureWriter from Context if possible,
		// or just execute its logic manually here.
	}
}

func (proxy *SushiProxy) RegisterRoutes(router chi.Router) {
	// Mount the proxy itself as the handler for all requests.
	// The proxy's ServeHTTP method will delegate to the internally managed dynamic router.
	router.Handle("/*", proxy)
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

// validateRequestFraming checks for HTTP request smuggling indicators
// Rejects requests with ambiguous Content-Length/Transfer-Encoding headers
func validateRequestFraming(req *http.Request) *model.HttpError {
	// 1. Check for multiple Content-Length headers
	if len(req.Header.Values("Content-Length")) > 1 {
		return model.NewHttpError(http.StatusBadRequest,
			"AMBIGUOUS_REQUEST_FRAMING",
			"Multiple Content-Length headers detected")
	}

	cl := req.Header.Get("Content-Length")

	// 2. Check for Transfer-Encoding headers that Go didn't consume (non-chunked)
	// Go moves "chunked" to req.TransferEncoding and removes it from the Header map.
	// If there are still values in the header map, they are unsupported or potentially malicious.
	if len(req.Header.Values("Transfer-Encoding")) > 0 {
		return model.NewHttpError(http.StatusNotImplemented,
			"UNSUPPORTED_TRANSFER_ENCODING",
			"Unsupported Transfer-Encoding header values")
	}

	// 3. Check for conflict between Content-Length and "chunked" Transfer-Encoding
	isChunked := false
	for _, enc := range req.TransferEncoding {
		if enc == "chunked" {
			isChunked = true
			break
		}
	}

	if isChunked && cl != "" {
		slog.Warn("Rejecting request with ambiguous framing",
			"path", req.URL.Path,
			"content_length", cl,
			"transfer_encoding", "chunked")
		return model.NewHttpError(http.StatusBadRequest,
			"AMBIGUOUS_REQUEST_FRAMING",
			"Request contains both Content-Length and Transfer-Encoding: chunked")
	}

	// 4. Validate Content-Length format (digits only)
	if cl != "" {
		// Use regex to ensure only digits. fmt.Sscanf can be loose.
		matched, _ := regexp.MatchString("^[0-9]+$", cl)
		if !matched {
			return model.NewHttpError(http.StatusBadRequest,
				"INVALID_CONTENT_LENGTH",
				"Content-Length must be a non-negative integer")
		}
	}

	return nil
}

// sanitizeHeaders removes CR and LF characters from header values
func sanitizeHeaders(h http.Header) {
	for k, vv := range h {
		for i, v := range vv {
			if strings.ContainsAny(v, "\r\n") {
				h[k][i] = strings.Map(func(r rune) rune {
					if r == '\r' || r == '\n' {
						return -1
					}
					return r
				}, v)
			}
		}
	}
}

// RouteRequest is deprecated in favor of BuildRouterFromConfig
func (proxy *SushiProxy) RouteRequest() http.HandlerFunc {
	// Legacy fallback if needed, or we can just panic/log
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Legacy RouteRequest called - Router not configured correctly", http.StatusInternalServerError)
	}
}

func (s *SushiProxy) HandleProxyPass(w http.ResponseWriter, req *http.Request, matchedService *model.Service, matchedRoute *model.Route) *model.HttpError {
	// 1. Matched Service/Route passed via arguments

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
		selectedUpstream, proxyURL, cookieToSet, convErr := s.selectUpstreamWithRetry(matchedService, matchedRoute, req, retryHandle.GetFailedUpstreams())
		if convErr != nil {

			// If no upstream available, no point retrying immediately unless config changes, but checking config is handled by others.
			// Just return error.
			return convErr
		}

		if cookieToSet != nil {
			http.SetCookie(w, cookieToSet)
			slog.Debug("Injecting sticky session cookie", "name", cookieToSet.Name, "value", cookieToSet.Value)
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
		// Wrap transport with per-target circuit breaker
		proxy.Transport = &CircuitBreakerTransport{
			Target: selectedUpstream.Target,
			Base:   transport,
		}

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

			// Respect PreserveHost setting
			preserveHost := false
			if matchedRoute != nil && matchedRoute.PreserveHost != nil {
				preserveHost = *matchedRoute.PreserveHost
			}

			if !preserveHost {
				req.Host = target.Host
			}

			// Security: Sanitize headers to prevent CRLF injection
			sanitizeHeaders(req.Header)
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
	cacheKey := service.TransportCacheKey
	if cacheKey == "" {
		// Fallback for safety (e.g. unit tests not using config loader)
		cacheKey = fmt.Sprintf("%d-%d-%d-%v-%s-%s-%s",
			service.ConnectTimeout, service.ReadTimeout, service.WriteTimeout,
			service.TLS.Enabled, service.TLS.CaCertPath, service.TLS.CertPath, service.TLS.KeyPath)
	}

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
func (s *SushiProxy) selectUpstream(matchedService *model.Service, req *http.Request) (*model.UpstreamTarget, string, *http.Cookie, *model.HttpError) {
	// Fallback/Legacy helper. We probably shouldn't use it or it needs matchedRoute too.
	// But it's used in tests maybe?
	// It calls selectUpstreamWithRetry(..., nil, ..., nil).
	// We need to update its signature or make it panic?
	// Or pass nil route? If nil route, no tags.
	return s.selectUpstreamWithRetry(matchedService, nil, req, nil)
}

// selectUpstreamWithRetry selects upstream while excluding failed ones
// This implements Kong's failedAddresses pattern for intelligent retry routing
func (s *SushiProxy) selectUpstreamWithRetry(matchedService *model.Service, matchedRoute *model.Route, req *http.Request, failedUpstreams map[string]bool) (*model.UpstreamTarget, string, *http.Cookie, *model.HttpError) {
	// Need to get matchedRoute for tags. Currently logic expects it from util.
	// To fix this cleanly, we should update signature of selectUpstreamWithRetry too, or just use util here as fallback?
	// But util scan is slow.
	// Best to pass matchedRoute.
	// I will update signature in next step or use closure?
	// I cannot change signature easily across all calls?
	// Wait, selectUpstreamWithRetry is called by HandleProxyPass.
	// HandleProxyPass HAS matchedRoute now.
	// So I should pass it down.
	// But this tool is doing multiple replacements.
	// Let's keep signature compat for now? NO, I am updating HandleProxyPass too.
	// So I will update Step 272 (the call site) too!

	// 0. Service Discovery
	if matchedService.ServiceName != "" {
		// If DiscoveryProvider is set (or just default/any), look it up
		if container.Global != nil && container.Global.Registry != nil {
			resolvedURL, err := container.Global.Registry.GetServiceURL(matchedService.ServiceName)
			if err != nil {
				slog.Error("Service discovery failed", "service", matchedService.ServiceName, "error", err)
				return nil, "", nil, &model.HttpError{
					Code:     "SERVICE_DISCOVERY_ERROR",
					Message:  fmt.Sprintf("Failed to resolve service %s: %v", matchedService.ServiceName, err),
					HttpCode: http.StatusBadGateway,
				}
			}
			return nil, resolvedURL, nil, nil
		} else {
			slog.Warn("Service Name configured but no Registry initialized", "service", matchedService.ServiceName)
		}
	}

	// 0.1 Direct URL (Kong style)
	if matchedService.URL != "" {
		return nil, matchedService.URL, nil, nil
	}

	loadBalancer := NewLoadBalancer(GlobalHealthChecker)

	// Get client IP for load balancing algorithms that need it
	clientIP, _ := util.GetHostIp(req.RemoteAddr)

	// Extract tags from route (Subset Load Balancing)
	// Extract tags from route (Subset Load Balancing)
	var tags []string
	if matchedRoute != nil {
		tags = matchedRoute.UpstreamTags
	}

	// Select upstream, potentially excluding failed ones for retry tracking
	var upstreamIndex int
	var cookieToSet *http.Cookie

	if len(failedUpstreams) > 0 {
		// Use retry-aware selection that excludes failed upstreams
		upstreamIndex = loadBalancer.GetNextUpstreamWithRetry(*matchedService, clientIP, failedUpstreams, tags)
	} else {
		// Get upstream config for hash_on settings (used by consistent-hashing)
		upstreamConfig := GetUpstreamConfigForService(GetGlobalProxyConfig(), matchedService)
		// Use the enhanced load balancer that extracts hash values based on upstream config
		upstreamIndex, cookieToSet = loadBalancer.GetNextUpstreamWithRequest(*matchedService, req, upstreamConfig, tags)
	}

	if upstreamIndex == model.NoUpstreamsAvailable {
		return nil, "", nil, &model.HttpError{
			Code:     "ERROR_NO_UPSTREAMS_AVAILABLE",
			Message:  "No upstreams available for service: " + matchedService.Name,
			HttpCode: http.StatusServiceUnavailable,
		}
	}

	upstream := matchedService.Upstreams[upstreamIndex]

	// 2. Select the specific route that matched this request to determine strip_path behavior
	// (matchedRoute was already resolved above for tags)

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
	return &upstream, proxyURL, cookieToSet, nil
}

// stripMatchedPath removes the matched prefix from the request path
// Includes path normalization to prevent directory traversal attacks
func (s *SushiProxy) stripMatchedPath(path string, route *model.Route) string {
	// Find which path in the route matched
	for _, p := range route.Paths {
		if strings.HasPrefix(path, p) {
			stripped := strings.TrimPrefix(path, p)
			if !strings.HasPrefix(stripped, "/") {
				stripped = "/" + stripped
			}
			// Security: Normalize path to prevent traversal attacks
			stripped = normalizePath(stripped)
			return stripped
		}
	}
	// Fallback for deprecated single path
	if route.Path != "" && strings.HasPrefix(path, route.Path) {
		stripped := strings.TrimPrefix(path, route.Path)
		if !strings.HasPrefix(stripped, "/") {
			stripped = "/" + stripped
		}
		// Security: Normalize path to prevent traversal attacks
		stripped = normalizePath(stripped)
		return stripped
	}
	return normalizePath(path)
}

// normalizePath cleans a URL path to prevent directory traversal
// This removes ../ sequences and double slashes
func normalizePath(path string) string {
	// filepath.Clean handles .., ., and multiple slashes
	cleaned := filepath.Clean(path)
	// Ensure it starts with /
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	// Convert backslashes to forward slashes (Windows compatibility)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	return cleaned
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
