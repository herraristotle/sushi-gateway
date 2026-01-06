package gateway

import (
	"context"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// ConfigStore defines the interface for persisting and retrieving gateway configuration.
// It supports both full config read/write and granular CRUD operations for dynamic updates.
type ConfigStore interface {
	// Global Config (Dbless mode fallback/initialization)
	GetProxyConfig(ctx context.Context) (*model.ProxyConfig, error)
	SaveProxyConfig(ctx context.Context, config *model.ProxyConfig) error

	// Services
	ListServices(ctx context.Context) ([]model.Service, error)
	GetService(ctx context.Context, name string) (*model.Service, error)
	UpsertService(ctx context.Context, service model.Service) error
	DeleteService(ctx context.Context, name string) error

	// Routes
	ListRoutes(ctx context.Context, serviceName string) ([]model.Route, error)
	UpsertRoute(ctx context.Context, serviceName string, route model.Route) error
	DeleteRoute(ctx context.Context, serviceName string, routeName string) error

	// Upstreams
	ListUpstreams(ctx context.Context) ([]model.UpstreamConfig, error)
	UpsertUpstream(ctx context.Context, upstream model.UpstreamConfig) error
	DeleteUpstream(ctx context.Context, name string) error

	// Plugins (Global)
	ListGlobalPlugins(ctx context.Context) ([]model.PluginConfig, error)
	UpsertGlobalPlugin(ctx context.Context, plugin model.PluginConfig) error
	DeleteGlobalPlugin(ctx context.Context, pluginId string) error
}
