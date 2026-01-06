package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWAFPlugin_Execute(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]interface{}
		target         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid Request",
			config:         nil,
			target:         "/?q=hello",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "SQLi Blocked (Default)",
			config:         nil,
			target:         "/?q=UNION+SELECT",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "XSS Blocked (Default)",
			config:         nil,
			target:         "/?q=<script>",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Observation Mode - SQLi Allowed",
			config: map[string]interface{}{
				"observation_mode": true,
			},
			target:         "/?q=UNION+SELECT",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Disable SQLi Blocking",
			config: map[string]interface{}{
				"block_sqli": false,
			},
			target:         "/?q=UNION+SELECT",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Disable XSS Blocking",
			config: map[string]interface{}{
				"block_xss": false,
			},
			target:         "/?q=<script>",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := NewWAFPlugin(tt.config)
			handler := plugin.Handler.Execute(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", tt.target, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
