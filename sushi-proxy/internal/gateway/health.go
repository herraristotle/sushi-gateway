package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

type FailureType string

const (
	PassiveHttpFailure FailureType = "http"
	PassiveTcpFailure  FailureType = "tcp"
	PassiveTimeout     FailureType = "timeout"
)

// Health status of a service
// Healthy = Service is available
// Unhealthy = Service is not available
// NotAvailable = Service does not have health check endpoint turned on, or health check has not started yet
type HealthStatus string

const (
	Healthy      HealthStatus = "healthy"
	Unhealthy    HealthStatus = "unhealthy"
	NotAvailable HealthStatus = "not_available"
)

var GlobalHealthChecker = NewHealthChecker()

type UpstreamHealthState struct {
	Status               HealthStatus
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
	LastCheck            time.Time
}

// service -> upstream -> health state
type HealthChecker struct {
	serviceHealthMap map[string]map[string]*UpstreamHealthState
	mutex            sync.RWMutex
}

func NewHealthChecker() *HealthChecker {
	serviceHealthMap := make(map[string]map[string]*UpstreamHealthState)
	return &HealthChecker{
		serviceHealthMap: serviceHealthMap,
		mutex:            sync.RWMutex{},
	}
}

func (hc *HealthChecker) Initialize() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	for _, service := range GetGlobalProxyConfig().Services {
		if _, ok := hc.serviceHealthMap[service.Name]; !ok {
			hc.serviceHealthMap[service.Name] = make(map[string]*UpstreamHealthState)
		}
		for _, upstream := range service.Upstreams {
			hc.serviceHealthMap[service.Name][upstream.Id] = &UpstreamHealthState{
				Status: NotAvailable,
			}
		}
	}
}

func (hc *HealthChecker) UpdateHealthState(serviceName string, upstreamId string, state *UpstreamHealthState) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	if _, ok := hc.serviceHealthMap[serviceName]; !ok {
		hc.serviceHealthMap[serviceName] = make(map[string]*UpstreamHealthState)
	}
	hc.serviceHealthMap[serviceName][upstreamId] = state
}

func (hc *HealthChecker) GetHealthStatus(serviceName string, upstreamId string) HealthStatus {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if serviceMap, ok := hc.serviceHealthMap[serviceName]; ok {
		if state, ok := serviceMap[upstreamId]; ok {
			return state.Status
		}
	}
	return NotAvailable
}

// GetUpstreamHealth returns the full health state for an upstream (for API usage)
func (hc *HealthChecker) GetUpstreamHealth(serviceName string, upstreamId string) *UpstreamHealthState {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if serviceMap, ok := hc.serviceHealthMap[serviceName]; ok {
		if state, ok := serviceMap[upstreamId]; ok {
			return state
		}
	}
	return nil
}

func (hc *HealthChecker) CheckHealthForAllServices() {
	services := GetGlobalProxyConfig().Services
	var wg sync.WaitGroup

	slog.Info("Checking health for all services defined in proxy configuration...")
	for _, service := range services {
		// Use UpstreamHealthChecks if available (from upstream), otherwise fall back to Service.Health (legacy/simple)
		useDetailedChecks := service.UpstreamHealthChecks != nil && service.UpstreamHealthChecks.Active != nil

		// If neither detailed nor simple health checks are enabled, skip.
		if !useDetailedChecks && !service.Health.Enabled {
			continue
		}

		slog.Debug("Checking health for service: " + service.Name)

		for _, upstream := range service.Upstreams {
			wg.Add(1)
			s := service
			u := upstream

			go func(s *model.Service, u *model.UpstreamTarget) {
				defer wg.Done()

				// Determine config values
				healthPath := s.Health.Path
				interval := 5 * time.Second // Default interval
				healthyThreshold := 1
				unhealthyThreshold := 1

				if useDetailedChecks {
					active := s.UpstreamHealthChecks.Active
					if active.HttpPath != "" {
						healthPath = active.HttpPath
					}
					// Interval depends on current status
					currentState := hc.GetUpstreamHealthState(s.Name, u.Id)
					if currentState != nil {
						if currentState.Status == Healthy && active.Healthy != nil {
							if active.Healthy.Interval > 0 {
								interval = time.Duration(active.Healthy.Interval) * time.Second
							}
							if active.Healthy.Successes > 0 {
								healthyThreshold = active.Healthy.Successes
							}
						} else if currentState.Status == Unhealthy && active.Unhealthy != nil {
							if active.Unhealthy.Interval > 0 {
								interval = time.Duration(active.Unhealthy.Interval) * time.Second
							}
							if active.Unhealthy.HttpFailures > 0 {
								unhealthyThreshold = active.Unhealthy.HttpFailures
							}
						}
						// Check if it is time to run check
						if time.Since(currentState.LastCheck) < interval && !currentState.LastCheck.IsZero() {
							return
						}
					}
				}

				healthCheckPath := fmt.Sprintf("%s://%s%s", s.Protocol, u.Target, healthPath)

				// Perform Check
				timeout := 5 * time.Second
				if useDetailedChecks && s.UpstreamHealthChecks.Active.Timeout > 0 {
					timeout = time.Duration(s.UpstreamHealthChecks.Active.Timeout) * time.Second
				}

				client := http.Client{
					Timeout: timeout,
				}
				res, err := client.Get(healthCheckPath)
				success := err == nil && res.StatusCode == http.StatusOK
				if res != nil && res.Body != nil {
					res.Body.Close()
				}

				hc.processHealthResult(s, u.Id, success, healthyThreshold, unhealthyThreshold)

			}(&s, &u)
		}
	}
	wg.Wait()
}

func (hc *HealthChecker) GetUpstreamHealthState(serviceName string, upstreamId string) *UpstreamHealthState {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if serviceMap, ok := hc.serviceHealthMap[serviceName]; ok {
		return serviceMap[upstreamId]
	}
	return nil
}

func (hc *HealthChecker) processHealthResult(service *model.Service, upstreamId string, success bool, healthyThreshold int, unhealthyThreshold int) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if _, ok := hc.serviceHealthMap[service.Name]; !ok {
		hc.serviceHealthMap[service.Name] = make(map[string]*UpstreamHealthState)
	}
	state, ok := hc.serviceHealthMap[service.Name][upstreamId]
	if !ok {
		state = &UpstreamHealthState{Status: NotAvailable}
		hc.serviceHealthMap[service.Name][upstreamId] = state
	}

	state.LastCheck = time.Now()

	if success {
		state.ConsecutiveFailures = 0
		state.ConsecutiveSuccesses++
		if state.Status != Healthy && state.ConsecutiveSuccesses >= healthyThreshold {
			state.Status = Healthy
			slog.Info(fmt.Sprintf("Upstream %s for service %s is now HEALTHY", upstreamId, service.Name))
		}
	} else {
		state.ConsecutiveSuccesses = 0
		state.ConsecutiveFailures++
		if state.Status != Unhealthy && state.ConsecutiveFailures >= unhealthyThreshold {
			state.Status = Unhealthy
			slog.Info(fmt.Sprintf("Upstream %s for service %s is now UNHEALTHY", upstreamId, service.Name))
		}
	}
}

func (hc *HealthChecker) GetHealthyUpstreams(service model.Service) []model.UpstreamTarget {

	// Skip if health check is not enabled.
	isHealthCheckEnabled := service.Health.Enabled || (service.UpstreamHealthChecks != nil && service.UpstreamHealthChecks.Active != nil)
	if !isHealthCheckEnabled {
		return service.Upstreams
	}

	healthyUpstreams := make([]model.UpstreamTarget, 0)
	for _, upstream := range service.Upstreams {
		state := hc.GetUpstreamHealthState(service.Name, upstream.Id)
		if state != nil && state.Status == Healthy {
			healthyUpstreams = append(healthyUpstreams, upstream)
		}
	}

	return healthyUpstreams
}

func (hc *HealthChecker) RecordPassiveResult(service *model.Service, upstreamId string, success bool) {
	if service.UpstreamHealthChecks == nil || service.UpstreamHealthChecks.Passive == nil {
		return
	}

	// For now, we reuse the consecutive counters.
	// Passive health checks in Kong typically have their own counters,
	// but mapping them to the same HealthState is simpler for the initial implementation.

	healthyThreshold := 1
	unhealthyThreshold := 1

	if success {
		if service.UpstreamHealthChecks.Passive.Healthy != nil {
			healthyThreshold = service.UpstreamHealthChecks.Passive.Healthy.Successes
		}
	} else {
		if service.UpstreamHealthChecks.Passive.Unhealthy != nil {
			// We combine all types into HttpFailures for simplicity
			unhealthyThreshold = service.UpstreamHealthChecks.Passive.Unhealthy.HttpFailures
		}
	}

	if healthyThreshold <= 0 {
		healthyThreshold = 1
	}
	if unhealthyThreshold <= 0 {
		unhealthyThreshold = 1
	}

	hc.processHealthResult(service, upstreamId, success, healthyThreshold, unhealthyThreshold)
}
