package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// TODO: complete a simple e2e test for gateway.
func TestHandleProxyRequest(t *testing.T) {

	req, err := http.NewRequest("GET", "/mockService/mockRoute", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	proxy := NewSushiProxy()
	// Initialize with empty config -> empty router
	proxy.UpdateRouter(&model.ProxyConfig{})

	proxy.ServeHTTP(rr, req)
	// With empty router, this should be 404
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func Test_HandleServiceNotFound(t *testing.T) {
	req, err := http.NewRequest("GET", "/mockService/mockRoute", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	proxy := NewSushiProxy()
	proxy.UpdateRouter(&model.ProxyConfig{})

	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}

	// Chi returns "404 page not found" text by default, not JSON.
	// So we stop asserting JSON structure here.
}

func TestValidateRequestFraming(t *testing.T) {
	tests := []struct {
		name             string
		headers          map[string][]string
		transferEncoding []string
		wantErr          bool
	}{
		{
			name:    "Valid request",
			headers: map[string][]string{"Content-Length": {"10"}},
			wantErr: false,
		},
		{
			name:    "Multiple CL headers",
			headers: map[string][]string{"Content-Length": {"10", "11"}},
			wantErr: true,
		},
		{
			name:             "Conflicting CL and parsed chunked TE",
			headers:          map[string][]string{"Content-Length": {"10"}},
			transferEncoding: []string{"chunked"},
			wantErr:          true,
		},
		{
			name:    "Unparsed TE header in map (unsupported)",
			headers: map[string][]string{"Transfer-Encoding": {"gzip"}},
			wantErr: true,
		},
		{
			name:    "Invalid CL format",
			headers: map[string][]string{"Content-Length": {"-1"}},
			wantErr: true,
		},
		{
			name:    "Invalid CL chars",
			headers: map[string][]string{"Content-Length": {"10a"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header[k] = v
			}
			if len(tt.transferEncoding) > 0 {
				req.TransferEncoding = tt.transferEncoding
			}

			err := validateRequestFraming(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequestFraming() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
