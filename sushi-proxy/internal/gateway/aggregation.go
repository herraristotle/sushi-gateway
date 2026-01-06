package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
	"golang.org/x/sync/errgroup"
)

// AggregationHandler handles response aggregation for routes with multiple backends
type AggregationHandler struct {
	httpClient *http.Client
}

// BackendResult holds the result from a single backend call
type BackendResult struct {
	Name       string
	StatusCode int
	Body       json.RawMessage
	Error      error
	Duration   time.Duration
}

// AggregatedResponse is the final merged response sent to the client
type AggregatedResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []AggregationError         `json:"errors,omitempty"`
	Meta   AggregationMeta            `json:"_meta"`
}

// AggregationError represents an error from a single backend
type AggregationError struct {
	Backend string `json:"backend"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// AggregationMeta contains metadata about the aggregation
type AggregationMeta struct {
	TotalBackends    int              `json:"total_backends"`
	SuccessfulCalls  int              `json:"successful_calls"`
	FailedCalls      int              `json:"failed_calls"`
	TotalDurationMs  int64            `json:"total_duration_ms"`
	BackendDurations map[string]int64 `json:"backend_durations"`
}

// NewAggregationHandler creates a new aggregation handler
func NewAggregationHandler() *AggregationHandler {
	return &AggregationHandler{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// HandleAggregation executes all backend calls in parallel and merges responses
func (h *AggregationHandler) HandleAggregation(w http.ResponseWriter, r *http.Request, route *model.Route) {
	startTime := time.Now()
	ctx := r.Context()

	backends := route.Backends
	if len(backends) == 0 {
		http.Error(w, "No backends configured for aggregation", http.StatusInternalServerError)
		return
	}

	slog.Info("Starting response aggregation",
		"route", route.Name,
		"backends", len(backends))

	RecordAggregationRequest(route.Name)

	// Channel to collect results
	results := make([]BackendResult, len(backends))
	var mu sync.Mutex

	// Use errgroup for parallel execution with proper error handling
	g, ctx := errgroup.WithContext(ctx)

	for i, backend := range backends {
		i, backend := i, backend // Capture loop variables

		g.Go(func() error {
			result := h.callBackend(ctx, r, &backend)

			mu.Lock()
			results[i] = result
			mu.Unlock()

			// If backend is required and failed, cancel other requests
			if backend.Required && result.Error != nil {
				return fmt.Errorf("required backend %s failed: %w", backend.Name, result.Error)
			}

			return nil
		})
	}

	// Wait for all backends (or until a required one fails)
	requiredErr := g.Wait()

	// Build aggregated response
	response := h.buildAggregatedResponse(results, backends, route.Name, startTime, requiredErr)

	// Determine HTTP status code
	statusCode := http.StatusOK
	if requiredErr != nil {
		statusCode = http.StatusBadGateway
	} else if response.Meta.FailedCalls > 0 && response.Meta.SuccessfulCalls == 0 {
		statusCode = http.StatusBadGateway
	} else if response.Meta.FailedCalls > 0 {
		statusCode = http.StatusPartialContent // 206 - partial success
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Aggregation-Backends", fmt.Sprintf("%d", len(backends)))
	w.Header().Set("X-Aggregation-Success", fmt.Sprintf("%d", response.Meta.SuccessfulCalls))
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode aggregated response", "error", err)
	}

	slog.Info("Aggregation completed",
		"route", route.Name,
		"total_duration_ms", response.Meta.TotalDurationMs,
		"successful", response.Meta.SuccessfulCalls,
		"failed", response.Meta.FailedCalls)
}

// callBackend makes an HTTP request to a single backend
func (h *AggregationHandler) callBackend(ctx context.Context, originalReq *http.Request, backend *model.Backend) BackendResult {
	startTime := time.Now()

	result := BackendResult{
		Name: backend.Name,
	}

	// Build backend URL with path parameter substitution
	backendPath := h.substitutePath(backend.Path, originalReq)
	url := fmt.Sprintf("%s://%s%s", backend.Protocol, backend.Target, backendPath)

	// Create timeout context for this specific backend
	timeout := time.Duration(backend.ReadTimeout) * time.Millisecond
	if timeout == 0 {
		timeout = time.Duration(backend.TimeoutMs) * time.Millisecond
	}
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30s timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create new request
	req, err := http.NewRequestWithContext(ctx, originalReq.Method, url, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Copy relevant headers
	h.copyHeaders(originalReq, req)

	// Copy body for POST/PUT/PATCH
	if originalReq.Body != nil && (originalReq.Method == http.MethodPost ||
		originalReq.Method == http.MethodPut || originalReq.Method == http.MethodPatch) {
		bodyBytes, err := io.ReadAll(originalReq.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			// Restore original body for other backends
			originalReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	slog.Debug("Calling backend", "name", backend.Name, "url", url)

	// Make request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("request failed: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Duration = time.Since(startTime)

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to read response: %w", err)
		return result
	}

	// Ensure body is valid JSON, fallback to empty object if empty or non-JSON
	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 {
		trimmedBody = []byte("{}")
	} else if !json.Valid(trimmedBody) {
		slog.Warn("Backend returned invalid JSON, wrapping in string", "backend", backend.Name, "body", string(trimmedBody))
		// If not valid JSON, we must escape it as a string to be a valid RawMessage
		escaped, _ := json.Marshal(string(trimmedBody))
		trimmedBody = escaped
	}
	result.Body = json.RawMessage(trimmedBody)
	return result
}

// substitutePath replaces path parameters like {id} with values from the original request
func (h *AggregationHandler) substitutePath(backendPath string, originalReq *http.Request) string {
	// Extract path parameters from original URL
	// Example: /users/{id} -> /users/123
	re := regexp.MustCompile(`\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(backendPath, func(match string) string {
		paramName := strings.Trim(match, "{}")

		// Try to get from URL path (chi context)
		if value := util.GetPathParam(originalReq, paramName); value != "" {
			return value
		}

		// Try to get from query string
		if value := originalReq.URL.Query().Get(paramName); value != "" {
			return value
		}

		return match // Keep original if not found
	})
}

// copyHeaders copies relevant headers from original request to backend request
func (h *AggregationHandler) copyHeaders(from *http.Request, to *http.Request) {
	// Headers to forward
	headersToForward := []string{
		"Authorization",
		"Accept",
		"Accept-Language",
		"Content-Type",
		"X-Request-ID",
		"X-Correlation-ID",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"User-Agent",
	}

	for _, header := range headersToForward {
		if value := from.Header.Get(header); value != "" {
			to.Header.Set(header, value)
		}
	}
}

// buildAggregatedResponse constructs the final merged response
func (h *AggregationHandler) buildAggregatedResponse(
	results []BackendResult,
	backends []model.Backend,
	routeName string,
	startTime time.Time,
	requiredErr error,
) *AggregatedResponse {
	response := &AggregatedResponse{
		Data:   make(map[string]json.RawMessage),
		Errors: make([]AggregationError, 0),
		Meta: AggregationMeta{
			TotalBackends:    len(backends),
			BackendDurations: make(map[string]int64),
		},
	}

	for _, result := range results {
		response.Meta.BackendDurations[result.Name] = result.Duration.Milliseconds()

		if result.Error != nil {
			response.Meta.FailedCalls++
			RecordAggregationBackend(routeName, result.Name, false)
			response.Errors = append(response.Errors, AggregationError{
				Backend: result.Name,
				Message: result.Error.Error(),
				Code:    result.StatusCode,
			})
		} else {
			response.Meta.SuccessfulCalls++
			RecordAggregationBackend(routeName, result.Name, true)
			response.Data[result.Name] = result.Body
		}
	}

	response.Meta.TotalDurationMs = time.Since(startTime).Milliseconds()

	return response
}

// IsAggregationRoute checks if a route has backends configured for aggregation
func IsAggregationRoute(route *model.Route) bool {
	return len(route.Backends) > 0
}
