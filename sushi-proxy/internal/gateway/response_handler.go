package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

// This plugin is not configurable
// Used specially to capture response metadata in the Response Phase
type ResponseHandlerPlugin struct {
	config map[string]interface{}
}

func NewResponseHandlerPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_RESPONSE_HANDLER,
		Priority: 0,
		Handler: &ResponseHandlerPlugin{
			config: config,
		},
		Validator: ResponseHandlerPlugin{
			config: config,
		},
	}
}

func (plugin ResponseHandlerPlugin) Validate() error {
	return nil
}

func (plugin ResponseHandlerPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing response handler function...")
		next.ServeHTTP(w, r)

		// The writer passed in should already be our capture writer from SushiProxy,
		// but we also have it in context if needed.
		captureWriter, _ := w.(*captureResponseWriter)

		// After response is sent, add metadata to the request context
		// Add response metadata to request context after handling completes
		ctx := r.Context()
		ctx = context.WithValue(ctx, constant.CONTEXT_RESPONSE_HEADERS, captureWriter.Header())
		ctx = context.WithValue(ctx, constant.CONTEXT_RESPONSE_SIZE, captureWriter.size)
		ctx = context.WithValue(ctx, constant.CONTEXT_RESPONSE_STATUS, captureWriter.statusCode)
		ctx = context.WithValue(ctx, constant.CONTEXT_END_TIME, time.Now())
		*r = *r.WithContext(ctx)

		// Record Prometheus Metrics
		service, route, err := util.GetServiceAndRouteFromRequest(GetGlobalProxyConfig(), r)
		serviceName := "unknown"
		routeName := "unknown"
		if err == nil {
			serviceName = service.Name
			routeName = route.Name
		}

		startTime, ok := ctx.Value(constant.CONTEXT_START_TIME).(time.Time)
		// Fallback if not found (unlikely but safe)
		if !ok {
			startTime = time.Now()
		}
		duration := time.Since(startTime).Seconds()

		statusCode := 0
		if captureWriter != nil {
			statusCode = captureWriter.statusCode
		}

		slog.Info("Recording request metrics",
			"service", serviceName,
			"route", routeName,
			"method", r.Method,
			"status", statusCode,
			"duration", duration)
		RecordRequest(serviceName, routeName, r.Method, statusCode, duration)
	})
}
