package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestExtractHashValue_IP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		upstreamConfig *model.UpstreamConfig
		expectedHash   string
	}{
		{
			name:       "IP from RemoteAddr",
			remoteAddr: "192.168.1.100:54321",
			upstreamConfig: &model.UpstreamConfig{
				HashOn: model.HashOnIP,
			},
			expectedHash: "192.168.1.100",
		},
		{
			name:          "IP from X-Forwarded-For",
			remoteAddr:    "127.0.0.1:8080",
			xForwardedFor: "10.0.0.1, 10.0.0.2",
			upstreamConfig: &model.UpstreamConfig{
				HashOn: model.HashOnIP,
			},
			expectedHash: "10.0.0.1",
		},
		{
			name:       "IP from X-Real-IP",
			remoteAddr: "127.0.0.1:8080",
			xRealIP:    "172.16.0.1",
			upstreamConfig: &model.UpstreamConfig{
				HashOn: model.HashOnIP,
			},
			expectedHash: "172.16.0.1",
		},
		{
			name:           "Default to IP when config is nil",
			remoteAddr:     "192.168.1.50:12345",
			upstreamConfig: nil,
			expectedHash:   "192.168.1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			hash, ok := ExtractHashValue(req, tt.upstreamConfig)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestExtractHashValue_Header(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Session-ID", "session-abc-123")

	upstreamConfig := &model.UpstreamConfig{
		HashOn:       model.HashOnHeader,
		HashOnHeader: "X-Session-ID",
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	assert.Equal(t, "session-abc-123", hash)
}

func TestExtractHashValue_Cookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.AddCookie(&http.Cookie{Name: "user_session", Value: "cookie-value-xyz"})

	upstreamConfig := &model.UpstreamConfig{
		HashOn:       model.HashOnCookie,
		HashOnCookie: "user_session",
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	assert.Equal(t, "cookie-value-xyz", hash)
}

func TestExtractHashValue_Path(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/users/123", nil)
	req.RemoteAddr = "127.0.0.1:8080"

	upstreamConfig := &model.UpstreamConfig{
		HashOn: model.HashOnPath,
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	assert.Equal(t, "/api/v1/users/123", hash)
}

func TestExtractHashValue_QueryArg(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?tenant=acme&version=2", nil)
	req.RemoteAddr = "127.0.0.1:8080"

	upstreamConfig := &model.UpstreamConfig{
		HashOn:         model.HashOnQueryArg,
		HashOnQueryArg: "tenant",
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	assert.Equal(t, "acme", hash)
}

func TestExtractHashValue_Consumer(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:8080"

	// Set consumer ID in context (as authentication plugins would do)
	ctx := context.WithValue(req.Context(), constant.CONTEXT_CONSUMER_ID, "user-12345")
	req = req.WithContext(ctx)

	upstreamConfig := &model.UpstreamConfig{
		HashOn: model.HashOnConsumer,
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	assert.Equal(t, "user-12345", hash)
}

func TestExtractHashValue_Fallback(t *testing.T) {
	// Request with no matching header, should fallback to IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	upstreamConfig := &model.UpstreamConfig{
		HashOn:       model.HashOnHeader,
		HashOnHeader: "X-Missing-Header", // Header not present
		HashFallback: model.HashOnIP,     // Fallback to IP
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	// Should fallback to IP since header is missing
	assert.Equal(t, "192.168.1.1", hash)
}

func TestExtractHashValue_CookieFallbackToIP(t *testing.T) {
	// Request with no matching cookie, should fallback to IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.5:8080"

	upstreamConfig := &model.UpstreamConfig{
		HashOn:       model.HashOnCookie,
		HashOnCookie: "session_id", // Cookie not present
		HashFallback: model.HashOnIP,
	}

	hash, ok := ExtractHashValue(req, upstreamConfig)
	assert.True(t, ok)
	// Should fallback to IP since cookie is missing
	assert.Equal(t, "10.0.0.5", hash)
}

func TestGetUpstreamConfigForService(t *testing.T) {
	proxyConfig := &model.ProxyConfig{
		Upstreams: []model.UpstreamConfig{
			{
				Name:         "api-upstream",
				HashOn:       model.HashOnHeader,
				HashOnHeader: "X-User-ID",
			},
			{
				Name:         "web-upstream",
				HashOn:       model.HashOnCookie,
				HashOnCookie: "session",
			},
		},
	}

	tests := []struct {
		name           string
		service        *model.Service
		expectedConfig *model.UpstreamConfig
	}{
		{
			name: "Find matching upstream",
			service: &model.Service{
				Host: "api-upstream",
			},
			expectedConfig: &proxyConfig.Upstreams[0],
		},
		{
			name: "Find different upstream",
			service: &model.Service{
				Host: "web-upstream",
			},
			expectedConfig: &proxyConfig.Upstreams[1],
		},
		{
			name: "No matching upstream",
			service: &model.Service{
				Host: "unknown-upstream",
			},
			expectedConfig: nil,
		},
		{
			name: "Empty host",
			service: &model.Service{
				Host: "",
			},
			expectedConfig: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUpstreamConfigForService(proxyConfig, tt.service)
			if tt.expectedConfig == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedConfig.Name, result.Name)
			}
		})
	}
}
