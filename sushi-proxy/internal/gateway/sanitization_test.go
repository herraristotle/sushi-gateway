package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizationPlugin_Execute(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		url            string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name:           "Valid Request",
			method:         "GET",
			url:            "/api/test?q=hello",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "SQLi in Query Param - UNION SELECT",
			method:         "GET",
			url:            "/api/test?q=UNION%20SELECT%20*%20FROM%20users",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "SQLi in Query Param - 1=1",
			method:         "GET",
			url:            "/api/test?id=1%20OR%201=1",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "XSS in Query Param - Script Tag",
			method:         "GET",
			url:            "/api/test?q=<script>alert(1)</script>",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "XSS in Header",
			method: "GET",
			url:    "/api/test",
			headers: map[string]string{
				"X-Custom-Header": "<script>alert(1)</script>",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "Valid Header",
			method: "GET",
			url:    "/api/test",
			headers: map[string]string{
				"X-Custom-Header": "Just Text",
			},
			expectedStatus: http.StatusOK,
		},
	}

	plugin := NewSanitizationPlugin(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.url, nil)
			assert.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Mock capture writer if needed, usually passed in context but here we test the plugin logic directly
			// The plugin doesn't use capture writer, it writes directly to w or calls next

			rr := httptest.NewRecorder()

			// Dummy next handler
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := plugin.Handler.Execute(next)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestIsMalicious(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello world", false},
		{"UNION SELECT * FROM users", true},
		{"union select * from users", true},
		{"<script>alert(1)</script>", true},
		{"javascript:alert(1)", true},
		{"DROP TABLE users", true},
		{"1=1", true},
		{"select * from table", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, isMalicious(tt.input))
		})
	}
}
