package gateway

import (
	"context"
	"net/http"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

// HashExtractor extracts hash values from requests based on upstream configuration
// This enables flexible consistent-hashing on headers, cookies, paths, etc. (Kong parity)

// ExtractHashValue extracts a hash value from the request based on the hash_on configuration
// Returns the hash value string and whether extraction was successful
func ExtractHashValue(req *http.Request, upstream *model.UpstreamConfig) (string, bool) {
	if upstream == nil {
		// No upstream config, fall back to IP
		return extractIP(req), true
	}

	hashOn := upstream.HashOn
	if hashOn == "" || hashOn == model.HashOnNone {
		// Default to IP if not specified
		hashOn = model.HashOnIP
	}

	value, ok := extractByType(req, hashOn, upstream)
	if ok && value != "" {
		return value, true
	}

	// Try fallback if primary extraction failed
	if upstream.HashFallback != "" && upstream.HashFallback != model.HashOnNone {
		fallbackValue, fallbackOk := extractByType(req, upstream.HashFallback, upstream)
		if fallbackOk && fallbackValue != "" {
			return fallbackValue, true
		}
	}

	// Last resort: use IP
	return extractIP(req), true
}

// extractByType extracts value based on the hash type
func extractByType(req *http.Request, hashType model.HashOnType, upstream *model.UpstreamConfig) (string, bool) {
	switch hashType {
	case model.HashOnIP:
		return extractIP(req), true
	case model.HashOnConsumer:
		return extractConsumer(req), true
	case model.HashOnHeader:
		return extractHeader(req, upstream.HashOnHeader), true
	case model.HashOnCookie:
		return extractCookie(req, upstream.HashOnCookie), true
	case model.HashOnPath:
		return extractPath(req), true
	case model.HashOnQueryArg:
		return extractQueryArg(req, upstream.HashOnQueryArg), true
	default:
		return "", false
	}
}

// extractIP extracts the client IP address from the request
func extractIP(req *http.Request) string {
	// Check X-Forwarded-For first (for proxied requests)
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, err := util.GetHostIp(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return ip
}

// extractConsumer extracts the consumer ID from the request context
// This is set by authentication plugins (JWT, Key-Auth, etc.)
func extractConsumer(req *http.Request) string {
	if req.Context() == nil {
		return ""
	}

	consumerID := req.Context().Value(constant.CONTEXT_CONSUMER_ID)
	if consumerID == nil {
		return ""
	}

	if id, ok := consumerID.(string); ok {
		return id
	}
	return ""
}

// extractHeader extracts a specific header value from the request
func extractHeader(req *http.Request, headerName string) string {
	if headerName == "" {
		return ""
	}
	return req.Header.Get(headerName)
}

// extractCookie extracts a specific cookie value from the request
func extractCookie(req *http.Request, cookieName string) string {
	if cookieName == "" {
		return ""
	}
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// extractPath extracts the URL path from the request
func extractPath(req *http.Request) string {
	return req.URL.Path
}

// extractQueryArg extracts a specific query parameter from the request
func extractQueryArg(req *http.Request, paramName string) string {
	if paramName == "" {
		return ""
	}
	return req.URL.Query().Get(paramName)
}

// GetUpstreamConfigForService retrieves the UpstreamConfig associated with a service
// This links service -> upstream config to access hash_on settings
func GetUpstreamConfigForService(proxyConfig *model.ProxyConfig, service *model.Service) *model.UpstreamConfig {
	if proxyConfig == nil || service == nil {
		return nil
	}

	// Check if service has a Host field that references an upstream
	if service.Host == "" {
		return nil
	}

	// Look up the upstream by name
	for i := range proxyConfig.Upstreams {
		if proxyConfig.Upstreams[i].Name == service.Host {
			return &proxyConfig.Upstreams[i]
		}
	}

	return nil
}

// Helper to get hash value with context support
type hashContextKey string

const HashValueContextKey hashContextKey = "hash_value"

// SetHashValueInContext stores the hash value in context for later use
func SetHashValueInContext(ctx context.Context, hashValue string) context.Context {
	return context.WithValue(ctx, HashValueContextKey, hashValue)
}

// GetHashValueFromContext retrieves the hash value from context
func GetHashValueFromContext(ctx context.Context) string {
	if val := ctx.Value(HashValueContextKey); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
