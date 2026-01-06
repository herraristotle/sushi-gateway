package util

import (
	"net/http"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMatchRoute(t *testing.T) {
	tests := []struct {
		name        string
		route       model.Route
		requestPath string
		headers     map[string]string
		expected    bool
	}{
		{
			name: "Static match",
			route: model.Route{
				Path: "/foo",
			},
			requestPath: "/foo",
			expected:    true,
		},
		{
			name: "Static mis-match",
			route: model.Route{
				Path: "/foo",
			},
			requestPath: "/bar",
			expected:    false,
		},
		{
			name: "Header match success",
			route: model.Route{
				Path: "/foo",
				Headers: map[string]string{
					"x-canary": "true",
				},
			},
			requestPath: "/foo",
			headers: map[string]string{
				"x-canary": "true",
			},
			expected: true,
		},
		{
			name: "Header match fail",
			route: model.Route{
				Path: "/foo",
				Headers: map[string]string{
					"x-canary": "true",
				},
			},
			requestPath: "/foo",
			headers: map[string]string{
				"x-canary": "false",
			},
			expected: false,
		},
		{
			name: "Header missing in request",
			route: model.Route{
				Path: "/foo",
				Headers: map[string]string{
					"x-required": "yes",
				},
			},
			requestPath: "/foo",
			headers:     map[string]string{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost"+tt.requestPath, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := MatchRoute(&tt.route, tt.requestPath, req)
			assert.Equal(t, tt.expected, result)
		})
	}
}
