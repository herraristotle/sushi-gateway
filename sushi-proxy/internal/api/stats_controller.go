package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/gateway"
)

type StatsController struct{}

func NewStatsController() *StatsController {
	return &StatsController{}
}

func (c *StatsController) RegisterRoutes(router chi.Router) {
	router.Get("/api/health", c.GetHealthStatus)
	router.Get("/api/stats", c.GetStats)
}

// HealthStatus represents the health of an upstream
type HealthStatus struct {
	UpstreamId  string    `json:"upstream_id"`
	ServiceName string    `json:"service_name"`
	Target      string    `json:"target"`
	Status      string    `json:"status"`     // "healthy", "unhealthy"
	CheckType   string    `json:"check_type"` // "active", "passive", "both"
	LastChecked time.Time `json:"last_checked"`
	Successes   int       `json:"successes"`
	Failures    int       `json:"failures"`
}

// UpstreamStats represents metrics for an upstream
type UpstreamStats struct {
	UpstreamId   string   `json:"upstream_id"`
	ServiceName  string   `json:"service_name"`
	Target       string   `json:"target"`
	Weight       int      `json:"weight"`
	ActiveConns  int64    `json:"active_connections"`
	EWMALatency  float64  `json:"ewma_latency_ms"`
	HealthStatus string   `json:"health_status"`
	Tags         []string `json:"tags,omitempty"`
}

// RateLimitStats represents rate limiting statistics
type RateLimitStats struct {
	Scope   string  `json:"scope"` // "global", "service:name", etc.
	Hits    int64   `json:"hits"`
	Allowed int64   `json:"allowed"`
	HitRate float64 `json:"hit_rate"` // Percentage of hits
}

// GetHealthStatus returns health status for all upstreams
func (c *StatsController) GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	healthStatuses := []HealthStatus{}

	// Get all services from global config
	config := gateway.GetGlobalProxyConfig()
	if config == nil {
		http.Error(w, "Gateway config not loaded", http.StatusInternalServerError)
		return
	}

	healthChecker := gateway.GlobalHealthChecker
	if healthChecker == nil {
		// Return empty array if health checker not initialized
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthStatuses)
		return
	}

	// Iterate through services and their upstreams
	for _, service := range config.Services {
		for _, upstream := range service.Upstreams {
			status := &HealthStatus{
				UpstreamId:  upstream.Id,
				ServiceName: service.Name,
				Target:      upstream.Target,
				Status:      "unknown",
				CheckType:   "unknown",
				LastChecked: time.Now(),
			}

			// Get health state from health checker
			healthState := healthChecker.GetUpstreamHealth(service.Name, upstream.Id)
			if healthState != nil {
				status.Status = string(healthState.Status)
				status.Successes = healthState.ConsecutiveSuccesses
				status.Failures = healthState.ConsecutiveFailures
				status.LastChecked = healthState.LastCheck

				// Determine check type - simplified to avoid struct navigation issues
				if service.UpstreamHealthChecks != nil && service.UpstreamHealthChecks.Active != nil {
					status.CheckType = "active"
					if service.UpstreamHealthChecks.Passive != nil {
						status.CheckType = "both"
					}
				} else if service.UpstreamHealthChecks != nil && service.UpstreamHealthChecks.Passive != nil {
					status.CheckType = "passive"
				} else if service.Health.Enabled {
					// Legacy simple health check
					status.CheckType = "active"
				}
			}

			healthStatuses = append(healthStatuses, *status)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthStatuses)
}

// GetStats returns comprehensive statistics
func (c *StatsController) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := struct {
		Upstreams  []UpstreamStats  `json:"upstreams"`
		RateLimits []RateLimitStats `json:"rate_limits"`
	}{
		Upstreams:  []UpstreamStats{},
		RateLimits: []RateLimitStats{},
	}

	config := gateway.GetGlobalProxyConfig()
	if config == nil {
		http.Error(w, "Gateway config not loaded", http.StatusInternalServerError)
		return
	}

	healthChecker := gateway.GlobalHealthChecker

	// Collect upstream stats
	for _, service := range config.Services {
		for _, upstream := range service.Upstreams {
			upstreamStat := UpstreamStats{
				UpstreamId:   upstream.Id,
				ServiceName:  service.Name,
				Target:       upstream.Target,
				Weight:       upstream.Weight,
				ActiveConns:  gateway.GetActiveConnections(service.Name, upstream.Id),
				EWMALatency:  gateway.GetEWMA(service.Name, upstream.Id) * 1000, // Convert to ms
				HealthStatus: "unknown",
				Tags:         upstream.Tags,
			}

			// Get health status
			if healthChecker != nil {
				healthState := healthChecker.GetUpstreamHealth(service.Name, upstream.Id)
				if healthState != nil {
					upstreamStat.HealthStatus = string(healthState.Status)
				}
			}

			stats.Upstreams = append(stats.Upstreams, upstreamStat)
		}
	}

	// Note: Rate limit metrics would require additional tracking
	// For now returning empty array - can be populated later with Prometheus metrics

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
