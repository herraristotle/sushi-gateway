package gateway

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
	"gopkg.in/yaml.v3"
)

// ValidateAndParseSchema parses config from either JSON or YAML format.
// It auto-detects the format by trying JSON first, then falling back to YAML.
func ValidateAndParseSchema(raw []byte) (*model.ProxyConfig, error) {
	var config model.ProxyConfig

	// Try to detect format and parse accordingly
	// YAML is a superset of JSON, but we try JSON first for better error messages
	trimmed := bytes.TrimSpace(raw)

	// If it looks like JSON (starts with {), try JSON first
	if len(trimmed) > 0 && trimmed[0] == '{' {
		err := json.Unmarshal(raw, &config)
		if err == nil {
			slog.Info("Parsed config file as JSON")
			return &config, nil
		}
		slog.Debug("JSON parsing failed, trying YAML", "error", err)
	}

	// Try YAML (also handles JSON since YAML is a superset)
	err := yaml.Unmarshal(raw, &config)
	if err != nil {
		slog.Error("Error parsing config file (tried JSON and YAML)", "error", err)
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	slog.Info("Parsed config file as YAML")
	return &config, nil
}

func ValidateConfig(config *model.ProxyConfig) error {
	// Auto-generate IDs for entities that don't have them
	generateEntityIDs(config)

	if err := validateGeneralConfigs(config); err != nil {
		return err
	}

	if err := validatePlugins(config); err != nil {
		return err
	}

	if err := validateServices(config); err != nil {
		return err
	}

	if err := validateRoutes(config); err != nil {
		return err
	}

	return nil
}

// generateEntityIDs auto-generates IDs for plugins and upstreams that don't have them.
// This allows users to omit the 'id' field in configuration files.
func generateEntityIDs(config *model.ProxyConfig) {
	// Generate IDs for global plugins
	for i := range config.Plugins {
		if config.Plugins[i].Id == "" {
			config.Plugins[i].Id = generateID()
		}
	}

	// Generate IDs for top-level upstreams
	for i := range config.Upstreams {
		for j := range config.Upstreams[i].Targets {
			if config.Upstreams[i].Targets[j].Id == "" {
				config.Upstreams[i].Targets[j].Id = generateID()
			}
		}
	}

	// Generate IDs for service-level entities
	for i := range config.Services {
		// Service plugins
		for j := range config.Services[i].Plugins {
			if config.Services[i].Plugins[j].Id == "" {
				config.Services[i].Plugins[j].Id = generateID()
			}
		}

		// Upstreams
		for j := range config.Services[i].Upstreams {
			if config.Services[i].Upstreams[j].Id == "" {
				config.Services[i].Upstreams[j].Id = generateID()
			}
		}

		// Route plugins
		for j := range config.Services[i].Routes {
			for k := range config.Services[i].Routes[j].Plugins {
				if config.Services[i].Routes[j].Plugins[k].Id == "" {
					config.Services[i].Routes[j].Plugins[k].Id = generateID()
				}
			}
		}
	}

	slog.Debug("Auto-generated IDs for entities without IDs")
}

// generateID creates a random UUID-like identifier
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func validateGeneralConfigs(config *model.ProxyConfig) error {
	if config.Name == "" {
		return fmt.Errorf("global name is required")
	}
	return nil
}

func validatePlugins(config *model.ProxyConfig) error {
	// Aggregate plugins in the gateway
	var plugins []model.PluginConfig
	pluginValidator := NewPluginValidator()

	for _, globalPlugin := range config.Plugins {
		plugins = append(plugins, globalPlugin)
	}

	for _, service := range config.Services {
		for _, servicePlugin := range service.Plugins {
			plugins = append(plugins, servicePlugin)
		}

		for _, route := range service.Routes {
			for _, routePlugin := range route.Plugins {
				plugins = append(plugins, routePlugin)
			}
		}
	}

	// Validate each plugin
	for _, plugin := range plugins {
		err := pluginValidator.ValidatePlugin(plugin)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateServices checks that service names are unique.
// Note: base_path uniqueness is no longer strictly enforced to support Kong-style flat routing
// where multiple services might share the same base path (e.g. '/') and differentiate by route.
func validateServices(config *model.ProxyConfig) error {
	var serviceNames []string
	serviceValidator := NewServiceValidator()

	for _, service := range config.Services {
		// Name
		if util.SliceContainsString(serviceNames, service.Name) {
			return fmt.Errorf("service name: %s must be unique", service.Name)
		}

		// Generic service validations
		if err := serviceValidator.ValidateService(service); err != nil {
			return err
		}

		// TODO: add validation for service route paths and methods, they must be unique.

		serviceNames = append(serviceNames, service.Name)
	}
	return nil
}

func validateRoutes(config *model.ProxyConfig) error {

	for _, service := range config.Services {
		var routePaths []string
		var routeNames []string
		routeValidator := NewRouteValidator()

		for _, route := range service.Routes {
			// Name
			if util.SliceContainsString(routeNames, route.Name) {
				return fmt.Errorf("route name: %s must be unique", route.Name)
			}

			// Generic route validations
			if err := routeValidator.ValidateRoute(route); err != nil {
				return err
			}

			routePaths = append(routePaths, route.Path)
			routeNames = append(routeNames, route.Name)
		}
	}

	return nil
}
