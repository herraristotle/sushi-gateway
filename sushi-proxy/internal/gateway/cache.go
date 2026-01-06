package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// CachePlugin implements response caching with Redis backend
type CachePlugin struct {
	config map[string]interface{}
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	CachedAt   int64             `json:"cached_at"`
}

func NewCachePlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_CACHE,
		Priority: 100, // Run early to short-circuit cached responses
		Handler: CachePlugin{
			config: config,
		},
		Validator: CachePlugin{
			config: config,
		},
	}
}

func (plugin CachePlugin) Validate() error {
	ttl, ok := plugin.config["ttl"].(float64)
	if !ok {
		return fmt.Errorf("ttl must be a number (seconds)")
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be greater than 0")
	}

	return nil
}

// getTTL returns the cache TTL in seconds
func (plugin CachePlugin) getTTL() time.Duration {
	ttl, ok := plugin.config["ttl"].(float64)
	if !ok {
		return 60 * time.Second // Default 60 seconds
	}
	return time.Duration(ttl) * time.Second
}

// shouldRespectCacheControl returns whether to honor Cache-Control headers
func (plugin CachePlugin) shouldRespectCacheControl() bool {
	val, ok := plugin.config["cache_control"].(bool)
	if !ok {
		return true // Default to respecting Cache-Control
	}
	return val
}

// buildCacheKey generates a unique cache key for the request
func (plugin CachePlugin) buildCacheKey(r *http.Request) string {
	// Include method, host, path, and query string in cache key
	return CacheKey(r.Method, r.Host+r.URL.RequestURI())
}

// isCacheable determines if a response should be cached
func (plugin CachePlugin) isCacheable(statusCode int, headers http.Header) bool {
	// Only cache successful responses
	if statusCode < 200 || statusCode >= 300 {
		return false
	}

	// Check Cache-Control directives if configured
	if plugin.shouldRespectCacheControl() {
		cacheControl := headers.Get("Cache-Control")
		if strings.Contains(cacheControl, "no-store") ||
			strings.Contains(cacheControl, "no-cache") ||
			strings.Contains(cacheControl, "private") {
			return false
		}
	}

	return true
}

// getFromCache retrieves a cached response from Redis
func (plugin CachePlugin) getFromCache(ctx context.Context, key string) (*CachedResponse, bool) {
	if GlobalRedisClient == nil {
		return nil, false
	}

	data, err := GlobalRedisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		slog.Warn("Failed to unmarshal cached response", "error", err)
		return nil, false
	}

	return &cached, true
}

// storeInCache stores a response in Redis
func (plugin CachePlugin) storeInCache(ctx context.Context, key string, response *CachedResponse, ttl time.Duration) {
	if GlobalRedisClient == nil {
		return
	}

	data, err := json.Marshal(response)
	if err != nil {
		slog.Warn("Failed to marshal response for caching", "error", err)
		return
	}

	if err := GlobalRedisClient.Set(ctx, key, data, ttl).Err(); err != nil {
		slog.Warn("Failed to store response in cache", "error", err)
	}
}

// cacheResponseWriter captures the response for caching
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	headers    http.Header
}

func newCacheResponseWriter(w http.ResponseWriter) *cacheResponseWriter {
	return &cacheResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		headers:        make(http.Header),
	}
}

func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	// Copy headers before writing
	for k, v := range w.ResponseWriter.Header() {
		w.headers[k] = v
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *cacheResponseWriter) Write(data []byte) (int, error) {
	// Write to both the actual response and our buffer
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// Execute implements the cache middleware
func (plugin CachePlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		cacheKey := plugin.buildCacheKey(r)
		ttl := plugin.getTTL()

		// Try cache lookup
		if cached, found := plugin.getFromCache(ctx, cacheKey); found {
			RecordCacheHit()
			slog.Info("Cache HIT", "key", cacheKey)
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Cache-Age", fmt.Sprintf("%d", time.Now().Unix()-cached.CachedAt))

			// Restore cached headers
			for k, v := range cached.Headers {
				w.Header().Set(k, v)
			}

			w.WriteHeader(cached.StatusCode)
			w.Write(cached.Body)
			return
		}

		RecordCacheMiss()
		slog.Info("Cache MISS", "key", cacheKey)
		w.Header().Set("X-Cache", "MISS")

		// Wrap response writer to capture response
		recorder := newCacheResponseWriter(w)

		// Call next handler
		next.ServeHTTP(recorder, r)

		// Store in cache if cacheable
		if plugin.isCacheable(recorder.statusCode, recorder.headers) {
			// Build cached response
			cachedHeaders := make(map[string]string)
			for k, v := range recorder.headers {
				if len(v) > 0 {
					cachedHeaders[k] = v[0]
				}
			}

			// Read body for caching
			body := recorder.body.Bytes()

			cached := &CachedResponse{
				StatusCode: recorder.statusCode,
				Headers:    cachedHeaders,
				Body:       body,
				CachedAt:   time.Now().Unix(),
			}

			go plugin.storeInCache(context.Background(), cacheKey, cached, ttl)
			slog.Info("Response cached", "key", cacheKey, "ttl", ttl)
		}
	})
}

// Helper function to read request body and restore it
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}
