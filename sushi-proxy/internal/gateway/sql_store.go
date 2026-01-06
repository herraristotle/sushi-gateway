package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

// ServiceDB represents the database model for a Service
type ServiceDB struct {
	gorm.Model
	Name                  string `gorm:"uniqueIndex"`
	URL                   string
	ServiceName           string
	DiscoveryProvider     string
	BasePath              string
	Protocol              string
	Host                  string
	LoadBalancingStrategy string
	HealthJSON            string
	RetryOptionsJSON      string
	TLSJSON               string
	PluginsJSON           string
}

// RouteDB represents the database model for a Route
type RouteDB struct {
	gorm.Model
	Name         string `gorm:"uniqueIndex"`
	ServiceName  string `gorm:"index"`
	PathsJSON    string
	MethodsJSON  string
	HeadersJSON  string
	PluginsJSON  string
	BackendsJSON string
	StripPath    *bool `gorm:"default:true"`
}

// UpstreamDB represents the database model for an Upstream
type UpstreamDB struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex"`
	Algorithm   string
	TargetsJSON string
}

// GlobalPluginDB represents the database model for a Global Plugin
type GlobalPluginDB struct {
	gorm.Model
	PluginID string `gorm:"uniqueIndex"`
	Name     string
	Enabled  *bool
	Config   string
}

// NewSQLStore creates a new SQLStore and runs auto-migration
func NewSQLStore(dbPath string) (*SQLStore, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Auto Migrate
	err = db.AutoMigrate(&ServiceDB{}, &RouteDB{}, &UpstreamDB{}, &GlobalPluginDB{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &SQLStore{db: db}, nil
}

// GetProxyConfig reconstructs the full model.ProxyConfig from the database
func (s *SQLStore) GetProxyConfig(ctx context.Context) (*model.ProxyConfig, error) {
	config := &model.ProxyConfig{
		Name: "sushi-gateway-sql",
	}

	// Load Upstreams
	var upstreamsDB []UpstreamDB
	if err := s.db.WithContext(ctx).Find(&upstreamsDB).Error; err != nil {
		return nil, err
	}
	for _, uDB := range upstreamsDB {
		u := model.UpstreamConfig{
			Name:      uDB.Name,
			Algorithm: model.LoadBalancingAlgorithm(uDB.Algorithm),
		}
		if err := json.Unmarshal([]byte(uDB.TargetsJSON), &u.Targets); err != nil {
			return nil, err
		}
		config.Upstreams = append(config.Upstreams, u)
	}

	// Load Global Plugins
	var pluginsDB []GlobalPluginDB
	if err := s.db.WithContext(ctx).Find(&pluginsDB).Error; err != nil {
		return nil, err
	}
	for _, pDB := range pluginsDB {
		p := model.PluginConfig{
			Id:      pDB.PluginID,
			Name:    pDB.Name,
			Enabled: pDB.Enabled,
		}
		if err := json.Unmarshal([]byte(pDB.Config), &p.Config); err != nil {
			return nil, err
		}
		config.Plugins = append(config.Plugins, p)
	}

	// Load Services and their Routes
	var servicesDB []ServiceDB
	if err := s.db.WithContext(ctx).Find(&servicesDB).Error; err != nil {
		return nil, err
	}
	for _, sDB := range servicesDB {
		svc := model.Service{
			Name:                  sDB.Name,
			URL:                   sDB.URL,
			ServiceName:           sDB.ServiceName,
			DiscoveryProvider:     sDB.DiscoveryProvider,
			BasePath:              sDB.BasePath,
			Protocol:              sDB.Protocol,
			Host:                  sDB.Host,
			LoadBalancingStrategy: model.LoadBalancingAlgorithm(sDB.LoadBalancingStrategy),
		}
		json.Unmarshal([]byte(sDB.HealthJSON), &svc.Health)
		json.Unmarshal([]byte(sDB.RetryOptionsJSON), &svc.RetryOptions)
		json.Unmarshal([]byte(sDB.TLSJSON), &svc.TLS)
		json.Unmarshal([]byte(sDB.PluginsJSON), &svc.Plugins)

		// Load Routes for this service
		var routesDB []RouteDB
		if err := s.db.WithContext(ctx).Where("service_name = ?", sDB.Name).Find(&routesDB).Error; err != nil {
			return nil, err
		}
		for _, rDB := range routesDB {
			r := model.Route{
				Name:      rDB.Name,
				StripPath: rDB.StripPath,
			}
			json.Unmarshal([]byte(rDB.PathsJSON), &r.Paths)
			json.Unmarshal([]byte(rDB.MethodsJSON), &r.Methods)
			json.Unmarshal([]byte(rDB.HeadersJSON), &r.Headers)
			json.Unmarshal([]byte(rDB.PluginsJSON), &r.Plugins)
			json.Unmarshal([]byte(rDB.BackendsJSON), &r.Backends)
			svc.Routes = append(svc.Routes, r)
		}
		config.Services = append(config.Services, svc)
	}

	return config, nil
}

// SaveProxyConfig wipes the current DB and saves the new configuration
func (s *SQLStore) SaveProxyConfig(ctx context.Context, config *model.ProxyConfig) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Clean start
		tx.Exec("DELETE FROM service_dbs")
		tx.Exec("DELETE FROM route_dbs")
		tx.Exec("DELETE FROM upstream_dbs")
		tx.Exec("DELETE FROM global_plugin_dbs")

		for _, u := range config.Upstreams {
			targetsJSON, _ := json.Marshal(u.Targets)
			tx.Create(&UpstreamDB{
				Name:        u.Name,
				Algorithm:   string(u.Algorithm),
				TargetsJSON: string(targetsJSON),
			})
		}

		for _, p := range config.Plugins {
			configJSON, _ := json.Marshal(p.Config)
			tx.Create(&GlobalPluginDB{
				PluginID: p.Id,
				Name:     p.Name,
				Enabled:  p.Enabled,
				Config:   string(configJSON),
			})
		}

		for _, svc := range config.Services {
			healthJSON, _ := json.Marshal(svc.Health)
			retryJSON, _ := json.Marshal(svc.RetryOptions)
			tlsJSON, _ := json.Marshal(svc.TLS)
			pluginsJSON, _ := json.Marshal(svc.Plugins)

			tx.Create(&ServiceDB{
				Name:                  svc.Name,
				URL:                   svc.URL,
				ServiceName:           svc.ServiceName,
				DiscoveryProvider:     svc.DiscoveryProvider,
				BasePath:              svc.BasePath,
				Protocol:              svc.Protocol,
				Host:                  svc.Host,
				LoadBalancingStrategy: string(svc.LoadBalancingStrategy),
				HealthJSON:            string(healthJSON),
				RetryOptionsJSON:      string(retryJSON),
				TLSJSON:               string(tlsJSON),
				PluginsJSON:           string(pluginsJSON),
			})

			for _, r := range svc.Routes {
				// Migrate legacy Path to Paths if needed
				if len(r.Paths) == 0 && r.Path != "" {
					r.Paths = []string{r.Path}
				}

				pathsJSON, _ := json.Marshal(r.Paths)
				methodsJSON, _ := json.Marshal(r.Methods)
				headersJSON, _ := json.Marshal(r.Headers)
				pluginsJSON, _ := json.Marshal(r.Plugins)
				backendsJSON, _ := json.Marshal(r.Backends)

				tx.Create(&RouteDB{
					Name:         r.Name,
					ServiceName:  svc.Name,
					PathsJSON:    string(pathsJSON),
					MethodsJSON:  string(methodsJSON),
					HeadersJSON:  string(headersJSON),
					PluginsJSON:  string(pluginsJSON),
					BackendsJSON: string(backendsJSON),
					StripPath:    r.StripPath,
				})
			}
		}
		return nil
	})
}

// ListServices returns all services from the database
func (s *SQLStore) ListServices(ctx context.Context) ([]model.Service, error) {
	config, err := s.GetProxyConfig(ctx)
	if err != nil {
		return nil, err
	}
	return config.Services, nil
}

// GetService returns a single service by name
func (s *SQLStore) GetService(ctx context.Context, name string) (*model.Service, error) {
	var sDB ServiceDB
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&sDB).Error; err != nil {
		return nil, err
	}
	svc := model.Service{
		Name:                  sDB.Name,
		URL:                   sDB.URL,
		ServiceName:           sDB.ServiceName,
		DiscoveryProvider:     sDB.DiscoveryProvider,
		BasePath:              sDB.BasePath,
		Protocol:              sDB.Protocol,
		Host:                  sDB.Host,
		LoadBalancingStrategy: model.LoadBalancingAlgorithm(sDB.LoadBalancingStrategy),
	}
	json.Unmarshal([]byte(sDB.HealthJSON), &svc.Health)
	json.Unmarshal([]byte(sDB.RetryOptionsJSON), &svc.RetryOptions)
	json.Unmarshal([]byte(sDB.TLSJSON), &svc.TLS)
	json.Unmarshal([]byte(sDB.PluginsJSON), &svc.Plugins)

	var routesDB []RouteDB
	if err := s.db.WithContext(ctx).Where("service_name = ?", name).Find(&routesDB).Error; err != nil {
		return nil, err
	}
	for _, rDB := range routesDB {
		r := model.Route{Name: rDB.Name}
		json.Unmarshal([]byte(rDB.PathsJSON), &r.Paths)
		json.Unmarshal([]byte(rDB.MethodsJSON), &r.Methods)
		json.Unmarshal([]byte(rDB.HeadersJSON), &r.Headers)
		json.Unmarshal([]byte(rDB.PluginsJSON), &r.Plugins)
		json.Unmarshal([]byte(rDB.BackendsJSON), &r.Backends)
		svc.Routes = append(svc.Routes, r)
	}
	return &svc, nil
}

// UpsertService creates or updates a service
func (s *SQLStore) UpsertService(ctx context.Context, service model.Service) error {
	healthJSON, _ := json.Marshal(service.Health)
	retryJSON, _ := json.Marshal(service.RetryOptions)
	tlsJSON, _ := json.Marshal(service.TLS)
	pluginsJSON, _ := json.Marshal(service.Plugins)

	sDB := ServiceDB{
		Name:                  service.Name,
		URL:                   service.URL,
		ServiceName:           service.ServiceName,
		DiscoveryProvider:     service.DiscoveryProvider,
		BasePath:              service.BasePath,
		Protocol:              service.Protocol,
		Host:                  service.Host,
		LoadBalancingStrategy: string(service.LoadBalancingStrategy),
		HealthJSON:            string(healthJSON),
		RetryOptionsJSON:      string(retryJSON),
		TLSJSON:               string(tlsJSON),
		PluginsJSON:           string(pluginsJSON),
	}

	return s.db.WithContext(ctx).Where("name = ?", service.Name).Assign(sDB).FirstOrCreate(&sDB).Error
}

// DeleteService removes a service and its associated routes
func (s *SQLStore) DeleteService(ctx context.Context, name string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().WithContext(ctx).Where("service_name = ?", name).Delete(&RouteDB{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().WithContext(ctx).Where("name = ?", name).Delete(&ServiceDB{}).Error
	})
}

// ListRoutes returns all routes for a given service
func (s *SQLStore) ListRoutes(ctx context.Context, serviceName string) ([]model.Route, error) {
	var routesDB []RouteDB
	if err := s.db.WithContext(ctx).Where("service_name = ?", serviceName).Find(&routesDB).Error; err != nil {
		return nil, err
	}
	var routes []model.Route
	for _, rDB := range routesDB {
		r := model.Route{Name: rDB.Name}
		json.Unmarshal([]byte(rDB.PathsJSON), &r.Paths)
		json.Unmarshal([]byte(rDB.MethodsJSON), &r.Methods)
		json.Unmarshal([]byte(rDB.HeadersJSON), &r.Headers)
		json.Unmarshal([]byte(rDB.PluginsJSON), &r.Plugins)
		json.Unmarshal([]byte(rDB.BackendsJSON), &r.Backends)
		routes = append(routes, r)
	}
	return routes, nil
}

// UpsertRoute creates or updates a route for a service
func (s *SQLStore) UpsertRoute(ctx context.Context, serviceName string, route model.Route) error {
	// Migrate legacy Path to Paths if needed
	if len(route.Paths) == 0 && route.Path != "" {
		route.Paths = []string{route.Path}
	}

	pathsJSON, _ := json.Marshal(route.Paths)
	methodsJSON, _ := json.Marshal(route.Methods)
	headersJSON, _ := json.Marshal(route.Headers)
	pluginsJSON, _ := json.Marshal(route.Plugins)
	backendsJSON, _ := json.Marshal(route.Backends)

	rDB := RouteDB{
		Name:         route.Name,
		ServiceName:  serviceName,
		PathsJSON:    string(pathsJSON),
		MethodsJSON:  string(methodsJSON),
		HeadersJSON:  string(headersJSON),
		PluginsJSON:  string(pluginsJSON),
		BackendsJSON: string(backendsJSON),
	}

	return s.db.WithContext(ctx).Where("name = ?", route.Name).Assign(rDB).FirstOrCreate(&rDB).Error
}

// DeleteRoute removes a route by name
func (s *SQLStore) DeleteRoute(ctx context.Context, serviceName string, routeName string) error {
	return s.db.Unscoped().WithContext(ctx).Where("service_name = ? AND name = ?", serviceName, routeName).Delete(&RouteDB{}).Error
}

// ListUpstreams returns all upstreams
func (s *SQLStore) ListUpstreams(ctx context.Context) ([]model.UpstreamConfig, error) {
	var upstreamsDB []UpstreamDB
	if err := s.db.WithContext(ctx).Find(&upstreamsDB).Error; err != nil {
		return nil, err
	}
	var upstreams []model.UpstreamConfig
	for _, uDB := range upstreamsDB {
		u := model.UpstreamConfig{
			Name:      uDB.Name,
			Algorithm: model.LoadBalancingAlgorithm(uDB.Algorithm),
		}
		json.Unmarshal([]byte(uDB.TargetsJSON), &u.Targets)
		upstreams = append(upstreams, u)
	}
	return upstreams, nil
}

// UpsertUpstream creates or updates an upstream
func (s *SQLStore) UpsertUpstream(ctx context.Context, upstream model.UpstreamConfig) error {
	targetsJSON, _ := json.Marshal(upstream.Targets)
	uDB := UpstreamDB{
		Name:        upstream.Name,
		Algorithm:   string(upstream.Algorithm),
		TargetsJSON: string(targetsJSON),
	}
	return s.db.WithContext(ctx).Where("name = ?", upstream.Name).Assign(uDB).FirstOrCreate(&uDB).Error
}

// DeleteUpstream removes an upstream
func (s *SQLStore) DeleteUpstream(ctx context.Context, name string) error {
	return s.db.Unscoped().WithContext(ctx).Where("name = ?", name).Delete(&UpstreamDB{}).Error
}

// ListGlobalPlugins returns all global plugins
func (s *SQLStore) ListGlobalPlugins(ctx context.Context) ([]model.PluginConfig, error) {
	var pluginsDB []GlobalPluginDB
	if err := s.db.WithContext(ctx).Find(&pluginsDB).Error; err != nil {
		return nil, err
	}
	var plugins []model.PluginConfig
	for _, pDB := range pluginsDB {
		p := model.PluginConfig{
			Id:      pDB.PluginID,
			Name:    pDB.Name,
			Enabled: pDB.Enabled,
		}
		json.Unmarshal([]byte(pDB.Config), &p.Config)
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// UpsertGlobalPlugin creates or updates a global plugin
func (s *SQLStore) UpsertGlobalPlugin(ctx context.Context, plugin model.PluginConfig) error {
	configJSON, _ := json.Marshal(plugin.Config)
	pDB := GlobalPluginDB{
		PluginID: plugin.Id,
		Name:     plugin.Name,
		Enabled:  plugin.Enabled,
		Config:   string(configJSON),
	}
	return s.db.WithContext(ctx).Where("plugin_id = ?", plugin.Id).Assign(pDB).FirstOrCreate(&pDB).Error
}

// DeleteGlobalPlugin removes a global plugin
func (s *SQLStore) DeleteGlobalPlugin(ctx context.Context, pluginId string) error {
	return s.db.Unscoped().WithContext(ctx).Where("plugin_id = ?", pluginId).Delete(&GlobalPluginDB{}).Error
}
