package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestHandleAggregation_BodyRaceCondition(t *testing.T) {
	// 1. Setup mock backends that read the body
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Echo body length
		resp := map[string]int{"len": len(body)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer backendServer.Close()

	// Parse host/port
	url := strings.TrimPrefix(backendServer.URL, "http://")
	hostPort := strings.Split(url, ":")

	// 2. Configure aggregation route with multiple concurrent backends hitting the same endpoint
	route := &model.Route{
		Name: "agg-race-test",
		Backends: []model.Backend{
			{Name: "b1", Target: hostPort[0] + ":" + hostPort[1], Protocol: "http", Path: "/"},
			{Name: "b2", Target: hostPort[0] + ":" + hostPort[1], Protocol: "http", Path: "/"},
			{Name: "b3", Target: hostPort[0] + ":" + hostPort[1], Protocol: "http", Path: "/"},
			{Name: "b4", Target: hostPort[0] + ":" + hostPort[1], Protocol: "http", Path: "/"},
			{Name: "b5", Target: hostPort[0] + ":" + hostPort[1], Protocol: "http", Path: "/"},
		},
	}

	// 3. Create request with significant body
	bodyContent := strings.Repeat("a", 1024*100) // 100KB body
	req := httptest.NewRequest("POST", "/aggregate", strings.NewReader(bodyContent))
	w := httptest.NewRecorder()

	// 4. Run handler
	handler := NewAggregationHandler()
	handler.HandleAggregation(w, req, route)

	// 5. Verify results
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	var resp AggregatedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Meta.SuccessfulCalls != 5 {
		t.Errorf("Expected 5 successful calls, got %d", resp.Meta.SuccessfulCalls)
	}

	// Verify each backend received the full body
	for name, data := range resp.Data {
		var result map[string]int
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("Failed to parse backend %s result: %v", name, err)
			continue
		}
		if result["len"] != len(bodyContent) {
			t.Errorf("Backend %s received partial body: got %d, want %d",
				name, result["len"], len(bodyContent))
		}
	}
}
