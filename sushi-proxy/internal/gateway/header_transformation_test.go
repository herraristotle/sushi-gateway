package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaderTransformationPlugin_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "no operations configured",
			config:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "valid add config",
			config: map[string]interface{}{
				"add": map[string]interface{}{
					"X-Custom-Header": "custom-value",
				},
			},
			wantErr: false,
		},
		{
			name: "valid remove config",
			config: map[string]interface{}{
				"remove": []interface{}{"X-Remove-Me"},
			},
			wantErr: false,
		},
		{
			name: "valid rename config",
			config: map[string]interface{}{
				"rename": map[string]interface{}{
					"X-Old-Name": "X-New-Name",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid add config (not a map)",
			config: map[string]interface{}{
				"add": "not-a-map",
			},
			wantErr: true,
		},
		{
			name: "invalid remove config (not an array)",
			config: map[string]interface{}{
				"remove": "not-an-array",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := HeaderTransformationPlugin{config: tt.config}
			err := plugin.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHeaderTransformationPlugin_Execute(t *testing.T) {
	tests := []struct {
		name            string
		config          map[string]interface{}
		inputHeaders    map[string]string
		expectedHeaders map[string]string
		removedHeaders  []string
	}{
		{
			name: "add header",
			config: map[string]interface{}{
				"add": map[string]interface{}{
					"X-Added": "added-value",
				},
			},
			inputHeaders: map[string]string{},
			expectedHeaders: map[string]string{
				"X-Added": "added-value",
			},
		},
		{
			name: "remove header",
			config: map[string]interface{}{
				"remove": []interface{}{"X-Remove-Me"},
			},
			inputHeaders: map[string]string{
				"X-Remove-Me": "should-be-removed",
				"X-Keep-Me":   "should-stay",
			},
			expectedHeaders: map[string]string{
				"X-Keep-Me": "should-stay",
			},
			removedHeaders: []string{"X-Remove-Me"},
		},
		{
			name: "rename header",
			config: map[string]interface{}{
				"rename": map[string]interface{}{
					"X-Old-Name": "X-New-Name",
				},
			},
			inputHeaders: map[string]string{
				"X-Old-Name": "my-value",
			},
			expectedHeaders: map[string]string{
				"X-New-Name": "my-value",
			},
			removedHeaders: []string{"X-Old-Name"},
		},
		{
			name: "replace header value",
			config: map[string]interface{}{
				"replace": map[string]interface{}{
					"X-Replace": "new-value",
				},
			},
			inputHeaders: map[string]string{
				"X-Replace": "old-value",
			},
			expectedHeaders: map[string]string{
				"X-Replace": "new-value",
			},
		},
		{
			name: "append to header",
			config: map[string]interface{}{
				"append": map[string]interface{}{
					"X-Append": "appended",
				},
			},
			inputHeaders: map[string]string{
				"X-Append": "original",
			},
			expectedHeaders: map[string]string{
				"X-Append": "original, appended",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := HeaderTransformationPlugin{config: tt.config}

			// Create a test request
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.inputHeaders {
				req.Header.Set(k, v)
			}

			// Create a test handler that captures the modified request
			var capturedReq *http.Request
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.WriteHeader(http.StatusOK)
			})

			// Execute the plugin
			handler := plugin.Execute(nextHandler)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Verify expected headers
			for k, v := range tt.expectedHeaders {
				assert.Equal(t, v, capturedReq.Header.Get(k), "header %s should have value %s", k, v)
			}

			// Verify removed headers
			for _, h := range tt.removedHeaders {
				assert.Empty(t, capturedReq.Header.Get(h), "header %s should be removed", h)
			}
		})
	}
}
