package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// Reads from declarative config file
var globalProxyConfig atomic.Pointer[model.ProxyConfig]
var GlobalConfigStore ConfigStore

func GetGlobalProxyConfig() *model.ProxyConfig {
	cfg := globalProxyConfig.Load()
	if cfg == nil {
		return &model.ProxyConfig{}
	}
	return cfg
}

func LoadProxyConfigFromConfigFile(filePath string) error {
	slog.Info("Loading proxy_pass gateway from config file.", "path", filePath)
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		slog.Error("Error reading gateway file", "error", err)
		return err
	}

	// Validate the gateway file, if valid -> assign to GlobalProxyConfig
	config, err := ValidateAndParseSchema(configFile)
	if err != nil {
		slog.Error("Error parsing gateway file", "error", err)
		return err
	}

	// Distribute plugins to their respective services and routes
	if err := distributePlugins(config); err != nil {
		slog.Error("Error distributing plugins", "error", err)
		return err
	}

	// Link upstreams to services
	if err := linkUpstreams(config); err != nil {
		slog.Error("Error linking upstreams", "error", err)
		return err
	}

	// Pre-compute runtime values
	initializeRuntime(config)

	err = ValidateConfig(config)
	if err != nil {
		slog.Error("Error validating gateway file", "error", err)
		return err
	}

	// If we have a SQL store, and it's empty, we might want to populate it from the file
	if GlobalConfigStore != nil {
		ctx := context.Background()
		existing, _ := GlobalConfigStore.ListServices(ctx)
		if len(existing) == 0 {
			slog.Info("SQL Store is empty, populating from config file...")
			if err := GlobalConfigStore.SaveProxyConfig(ctx, config); err != nil {
				slog.Error("Failed to sync config file to SQL store", "error", err)
			}
		}
	}

	slog.Info("Config file loaded successfully")
	// Validations passed
	globalProxyConfig.Store(config)

	// Reset load balancer caches
	ResetLoadBalancers()

	// Update Router dynamically
	if GlobalSushiProxy != nil {
		GlobalSushiProxy.UpdateRouter(config)
	}

	return nil
}

// InitSQLStore initializes the SQL-based configuration store
func InitSQLStore(dbPath string) error {
	store, err := NewSQLStore(dbPath)
	if err != nil {
		return err
	}
	GlobalConfigStore = store
	return nil
}

// ReloadConfigFromStore fetches config from the database and updates the gateway state
func ReloadConfigFromStore() error {
	if GlobalConfigStore == nil {
		return fmt.Errorf("ConfigStore not initialized")
	}

	ctx := context.Background()
	config, err := GlobalConfigStore.GetProxyConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch config from store: %w", err)
	}

	// Apply processing logic (plugins/upstreams)
	slog.Debug("Distributing plugins...")
	if err := distributePlugins(config); err != nil {
		slog.Error("Failed to distribute plugins during reload", "error", err)
		return err
	}
	slog.Debug("Linking upstreams...")
	if err := linkUpstreams(config); err != nil {
		slog.Error("Failed to link upstreams during reload", "error", err)
		return err
	}
	// Pre-compute runtime values
	initializeRuntime(config)
	slog.Debug("Validating config...")
	if err := ValidateConfig(config); err != nil {
		slog.Error("Failed to validate config during reload", "error", err)
		return err
	}

	globalProxyConfig.Store(config)
	ResetLoadBalancers()
	if GlobalSushiProxy != nil {
		GlobalSushiProxy.UpdateRouter(config)
	}
	slog.Info("Gateway configuration reloaded from SQL store")
	return nil
}

func WatchConfigFile(ctx context.Context, filePath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Error creating watcher for config file", "error", err)
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(filePath); err != nil {
		slog.Error("Error adding config file to watcher", "error", err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Config file watcher shutting down...")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Write != 0 {
				if err := LoadProxyConfigFromConfigFile(filePath); err != nil {
					// Log error but continue watching - don't terminate the watcher
					// Invalid config changes should not bring down the gateway
					slog.Error("Config reload failed, keeping previous config", "error", err)
					continue
				}
				slog.Info("Configuration reloaded successfully")
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			// Filesystem errors are also non-fatal, just log and continue
			slog.Error("Filesystem watcher error (non-fatal)", "error", err)
		}
	}
}

// distributePlugins iterates through top-level plugins and attaches them to targeted services/routes
// It then removes distributed plugins from the global config.Plugins list
func distributePlugins(config *model.ProxyConfig) error {
	var globalPlugins []model.PluginConfig

	for i := range config.Plugins {
		plugin := config.Plugins[i]
		distributed := false

		// 1. Target Service
		if plugin.Service != "" {
			found := false
			for sIdx := range config.Services {
				if config.Services[sIdx].Name == plugin.Service {
					config.Services[sIdx].Plugins = append(config.Services[sIdx].Plugins, plugin)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("plugin %s targets unknown service: %s", plugin.Name, plugin.Service)
			}
			distributed = true
		}

		// 2. Target Route
		if plugin.Route != "" {
			found := false
			for sIdx := range config.Services {
				for rIdx := range config.Services[sIdx].Routes {
					if config.Services[sIdx].Routes[rIdx].Name == plugin.Route {
						config.Services[sIdx].Routes[rIdx].Plugins = append(config.Services[sIdx].Routes[rIdx].Plugins, plugin)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf("plugin %s targets unknown route: %s", plugin.Name, plugin.Route)
			}
			distributed = true
		}

		// If not distributed to any service or route, keep it as global
		if !distributed {
			globalPlugins = append(globalPlugins, plugin)
		}
	}

	// update the global plugins list to only contain non-distributed plugins
	config.Plugins = globalPlugins

	return nil
}

// linkUpstreams connects services to their respective upstream targets based on the Host name
func linkUpstreams(config *model.ProxyConfig) error {
	for i := range config.Services {
		svc := &config.Services[i]

		// If Service has a Host defined, check if it matches a global Upstream name
		if svc.Host != "" {
			found := false
			for _, us := range config.Upstreams {
				if us.Name == svc.Host {
					// Found matching upstream, link targets
					svc.Upstreams = us.Targets
					// Kong Parity: Propagate algorithm from Upstream to Service strategy
					if us.Algorithm != "" {
						svc.LoadBalancingStrategy = us.Algorithm
					}
					// Kong Parity: Propagate detailed health checks
					if us.HealthChecks != nil {
						svc.UpstreamHealthChecks = us.HealthChecks
					}

					// Kong Parity: Propagate defaults and settings from Upstream if not set on Service
					if svc.ConnectTimeout == 0 && us.ConnectTimeout != 0 {
						svc.ConnectTimeout = us.ConnectTimeout
					}
					if svc.ReadTimeout == 0 && us.ReadTimeout != 0 {
						svc.ReadTimeout = us.ReadTimeout
					}
					if svc.WriteTimeout == 0 && us.WriteTimeout != 0 {
						svc.WriteTimeout = us.WriteTimeout
					}

					found = true
					break
				}
			}

			// Handle simplified Retries field mapping
			if svc.Retries != 0 && svc.RetryOptions.MaxRetries == 0 {
				svc.RetryOptions.MaxRetries = svc.Retries
			}

			// If not found in global upstreams, treat it as a direct single-host upstream?
			// For now, let's keep it strict or allow direct host.
			// If we want parity with Kong, a service 'host' that isn't an upstream is treated as a direct hostname.
			// If not found in global upstreams, treat it as a direct host.
			if !found {
				slog.Debug("Service host does not match any global upstream, treating as direct host", "service", svc.Name, "host", svc.Host)
				// Construct direct URL
				protocol := svc.Protocol
				if protocol == "" {
					protocol = "http"
				}
				svc.URL = fmt.Sprintf("%s://%s", protocol, svc.Host)
				// We don't overwrite svc.Upstreams here if they were already defined inline (legacy support)
				// But we should probably warn if both are empty.
			}
		}
	}
	return nil
}

// initializeRuntime pre-computes cache keys and other runtime optimizations
func initializeRuntime(config *model.ProxyConfig) {
	for i := range config.Services {
		svc := &config.Services[i]
		// Pre-compute Transport Cache Key to avoid Sprintf on hot path
		svc.TransportCacheKey = fmt.Sprintf("%d-%d-%d-%v-%s-%s-%s",
			svc.ConnectTimeout, svc.ReadTimeout, svc.WriteTimeout,
			svc.TLS.Enabled, svc.TLS.CaCertPath, svc.TLS.CertPath, svc.TLS.KeyPath)
	}
}
