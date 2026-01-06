package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

type RateLimitPlugin struct {
	config      map[string]interface{}
	proxyConfig *model.ProxyConfig
}

func NewRateLimitPlugin(config map[string]interface{}, proxyConfig *model.ProxyConfig) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_RATE_LIMIT,
		Priority: 50, // Run before cache to prevent rate limit bypass
		Handler: RateLimitPlugin{
			config:      config,
			proxyConfig: proxyConfig,
		},
		Validator: RateLimitPlugin{
			config: config,
		},
	}
}

// getNumberConfig safely extracts a number from config map handling float64, int, int64
func getNumberConfig(config map[string]interface{}, key string) (int64, error) {
	val, ok := config[key]
	if !ok {
		return 0, fmt.Errorf("missing key: %s", key)
	}

	switch v := val.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	default:
		return 0, fmt.Errorf("invalid type for key %s: expected number, got %T", key, val)
	}
}

func (plugin RateLimitPlugin) Validate() error {
	// Validate rate limits - at least one must be provided and > 0
	foundLimit := false
	limits := []string{"second", "minute", "hour"}

	for _, limitKey := range limits {
		if _, ok := plugin.config[limitKey]; ok {
			num, err := getNumberConfig(plugin.config, limitKey)
			if err != nil {
				return err
			}
			if num <= 0 {
				return fmt.Errorf("%s must be > 0", limitKey)
			}
			foundLimit = true
		}
	}

	if !foundLimit {
		return fmt.Errorf("at least one of [second, minute, hour] must be provided and > 0")
	}
	return nil
}

func (plugin RateLimitPlugin) detectRateLimitOperationLevel(service *model.Service, route *model.Route) string {
	// Check whether global, service or route level rate limit.
	for _, servicePlugin := range service.Plugins {
		name := servicePlugin.Name
		if name == constant.PLUGIN_RATE_LIMIT {
			return "Service"
		}
	}

	for _, routePlugin := range route.Plugins {
		name := routePlugin.Name
		if name == constant.PLUGIN_RATE_LIMIT {
			return "Route"
		}
	}

	return "Global"
}

func (plugin RateLimitPlugin) getMapKeyEntry(configLevel string, service *model.Service, route *model.Route) string {
	if configLevel == "Global" {
		return "Global"
	} else if configLevel == "Service" {
		return fmt.Sprintf("Service::%s", service.Name)
	} else {
		return fmt.Sprintf("Route::%s", route.Name)
	}
}

// checkRateLimitRedis checks and increments rate limit counter in Redis
// Returns true if request is allowed, false if rate limit exceeded
func (plugin RateLimitPlugin) checkRateLimitRedis(ctx context.Context, key string, limit int64, windowSeconds int) (bool, int64, error) {
	// Use Redis INCR + EXPIRE for atomic token bucket
	// The key includes the time window to create sliding windows
	windowKey := fmt.Sprintf("%s:%d", key, time.Now().Unix()/int64(windowSeconds))

	count, err := GlobalRedisClient.Incr(ctx, windowKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis INCR failed: %w", err)
	}

	// Set expiration only on first request (when count == 1)
	if count == 1 {
		GlobalRedisClient.Expire(ctx, windowKey, time.Duration(windowSeconds)*time.Second)
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= limit, remaining, nil
}

// Execute implementation using Redis for distributed rate limiting
// Uses Kong-style window-based rate limiting (fixed window default, sliding window available)
// Reference: https://developer.konghq.com/plugins/rate-limiting-advanced/
func (plugin RateLimitPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Executing rate limit function...")

		ctx := r.Context()

		// Get service and route from context (already matched by SushiProxy)
		serviceVal := ctx.Value(constant.CONTEXT_MATCHED_SERVICE)
		routeVal := ctx.Value(constant.CONTEXT_MATCHED_ROUTE)

		if serviceVal == nil || routeVal == nil {
			// Fallback to legacy lookup if context not set (e.g. in tests)
			service, route, err := util.GetServiceAndRouteFromRequest(plugin.proxyConfig, r)
			if err != nil {
				err.WriteLogMessage()
				err.WriteJSONResponse(w)
				return
			}
			serviceVal = service
			routeVal = route
		}

		service := serviceVal.(*model.Service)
		route := routeVal.(*model.Route)

		rateLimitOperationLevel := plugin.detectRateLimitOperationLevel(service, route)
		clientIp, err := util.GetHostIp(r.RemoteAddr)
		if err != nil {
			err.WriteLogMessage()
			err.WriteJSONResponse(w)
			return
		}

		// Get rate limits from config
		limitSec, _ := getNumberConfig(plugin.config, "second")
		limitMin, _ := getNumberConfig(plugin.config, "minute")
		limitHour, _ := getNumberConfig(plugin.config, "hour")

		// Advanced Config (Kong-compatible)
		faultTolerant := true
		if val, ok := plugin.config["fault_tolerant"].(bool); ok {
			faultTolerant = val
		}

		hideClientHeaders := false
		if val, ok := plugin.config["hide_client_headers"].(bool); ok {
			hideClientHeaders = val
		}

		// Get scope key
		scope := plugin.getMapKeyEntry(rateLimitOperationLevel, service, route)
		baseKey := RateLimitKey(fmt.Sprintf("%s:%s", scope, clientIp), "")

		// Helper to check limits using sliding window approach
		checkLimit := func(suffix string, limit int64, window int, headerSuffix string) bool {
			if limit == 0 {
				return true // Skip if limit is 0
			}

			key := baseKey + suffix
			allowed, remaining, redisIsErr := plugin.checkRateLimitRedis(ctx, key, limit, window)

			if redisIsErr != nil {
				slog.Error("Redis rate limit check failed", "error", redisIsErr)
				if !faultTolerant {
					model.NewHttpError(http.StatusInternalServerError, "RATE_LIMIT_STORE_ERROR", "An unexpected error occurred").WriteJSONResponse(w)
					return false
				}
				slog.Warn("Rate limiting bypassed due to Redis error (fault_tolerant=true)")
				return true
			}

			if !hideClientHeaders {
				w.Header().Set(fmt.Sprintf("X-RateLimit-Limit-%s", headerSuffix), fmt.Sprintf("%d", limit))
				w.Header().Set(fmt.Sprintf("X-RateLimit-Remaining-%s", headerSuffix), fmt.Sprintf("%d", remaining))
			}

			if !allowed {
				RecordRateLimitHit(scope, headerSuffix)
				if !hideClientHeaders {
					w.Header().Set(fmt.Sprintf("X-RateLimit-Reset-%s", headerSuffix), fmt.Sprintf("%d", window))
					w.Header().Set("Retry-After", fmt.Sprintf("%d", window))
				}
				httpErr := model.NewHttpError(http.StatusTooManyRequests,
					fmt.Sprintf("RATE_LIMIT_%s_EXCEEDED", strings.ToUpper(headerSuffix)),
					fmt.Sprintf("Rate limit exceeded for %s (per %s)", scope, headerSuffix))
				httpErr.WriteLogMessage()
				httpErr.WriteJSONResponse(w)
				return false
			}
			return true
		}

		// Check per-second limit
		if limitSec > 0 && !checkLimit("sec", limitSec, 1, "Second") {
			return
		}

		// Check per-minute limit
		if limitMin > 0 && !checkLimit("min", limitMin, 60, "Minute") {
			return
		}

		// Check per-hour limit
		if limitHour > 0 && !checkLimit("hr", limitHour, 3600, "Hour") {
			return
		}

		slog.Debug("Rate limiting passed", "ip", clientIp, "scope", scope)
		RecordRateLimitAllowed(scope)
		next.ServeHTTP(w, r)
	})
}
