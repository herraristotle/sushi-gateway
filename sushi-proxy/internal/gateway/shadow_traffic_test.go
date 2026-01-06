package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShadowTrafficPlugin_Execute(t *testing.T) {
	// 1. Setup Mock Shadow Backend
	shadowReceived := make(chan string, 1)
	shadowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shadowReceived <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer shadowServer.Close()

	// 2. Configure Plugin (Sync mode for testing)
	config := map[string]interface{}{
		"target_url":  shadowServer.URL,
		"sample_rate": 1.0,   // 100% sampling
		"async":       false, // Sync for deterministic test
	}

	pluginWrapper := NewShadowTrafficPlugin(config)
	plugin := pluginWrapper.Handler.(*ShadowTrafficPlugin)

	// 3. Create Request
	req := httptest.NewRequest("GET", "/test/path", nil)
	w := httptest.NewRecorder()

	// 4. Create Dummy Next Handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("main response"))
	})

	// 5. Execute
	handler := plugin.Execute(nextHandler)
	handler.ServeHTTP(w, req)

	// 6. Verify Main Response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "main response" {
		t.Errorf("Expected body 'main response', got '%s'", w.Body.String())
	}

	// 7. Verify Shadow Request Received
	select {
	case path := <-shadowReceived:
		if path != "/test/path" {
			t.Errorf("Expected shadow path /test/path, got %s", path)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for shadow request")
	}
}

func TestShadowTrafficPlugin_Sampling(t *testing.T) {
	// Configure with 0% sampling
	config := map[string]interface{}{
		"target_url":  "http://localhost:9999",
		"sample_rate": 0.0,
		"async":       false,
	}

	pluginWrapper := NewShadowTrafficPlugin(config)
	plugin := pluginWrapper.Handler.(*ShadowTrafficPlugin)

	// Create Handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := plugin.Execute(nextHandler)

	// Send request - should NOT shadow (otherwise would panic or timeout connecting to localhost:9999)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// This should succeed quickly and not try to connect
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
