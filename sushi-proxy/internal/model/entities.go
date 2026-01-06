package model

// Domain Model of objects in Sushi Gateway
type ProxyConfig struct {
	Name      string           `json:"name,omitempty" yaml:"name,omitempty"`
	Upstreams []UpstreamConfig `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`
	Plugins   []PluginConfig   `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Services  []Service        `json:"services" yaml:"services"`
}

type PluginConfig struct {
	Id      string                 `json:"id" yaml:"id"`
	Name    string                 `json:"name" yaml:"name"`
	Config  map[string]interface{} `json:"config" yaml:"config"`
	Enabled *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Service string                 `json:"service,omitempty" yaml:"service,omitempty"`
	Route   string                 `json:"route,omitempty" yaml:"route,omitempty"`
}

type UpstreamConfig struct {
	Name           string                 `json:"name" yaml:"name"`
	Algorithm      LoadBalancingAlgorithm `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	Targets        []UpstreamTarget       `json:"targets" yaml:"targets"`
	HealthChecks   *HealthCheckConfig     `json:"healthchecks,omitempty" yaml:"healthchecks,omitempty"`
	ConnectTimeout int                    `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	ReadTimeout    int                    `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty"`
	WriteTimeout   int                    `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty"`
	// Hash-based routing configuration (Kong parity)
	HashOn             HashOnType `json:"hash_on,omitempty" yaml:"hash_on,omitempty"`                           // none, consumer, ip, header, cookie, path, query_arg
	HashFallback       HashOnType `json:"hash_fallback,omitempty" yaml:"hash_fallback,omitempty"`               // Fallback if primary hash fails
	HashOnHeader       string     `json:"hash_on_header,omitempty" yaml:"hash_on_header,omitempty"`             // Header name when hash_on=header
	HashOnCookie       string     `json:"hash_on_cookie,omitempty" yaml:"hash_on_cookie,omitempty"`             // Cookie name when hash_on=cookie
	HashOnQueryArg     string     `json:"hash_on_query_arg,omitempty" yaml:"hash_on_query_arg,omitempty"`       // Query param when hash_on=query_arg
	HashFallbackHeader string     `json:"hash_fallback_header,omitempty" yaml:"hash_fallback_header,omitempty"` // Header for fallback
}

type HealthCheckConfig struct {
	Active  *ActiveHealthCheck  `json:"active,omitempty" yaml:"active,omitempty"`
	Passive *PassiveHealthCheck `json:"passive,omitempty" yaml:"passive,omitempty"`
}

type ActiveHealthCheck struct {
	Healthy     *HealthyConfig   `json:"healthy,omitempty" yaml:"healthy,omitempty"`
	Unhealthy   *UnhealthyConfig `json:"unhealthy,omitempty" yaml:"unhealthy,omitempty"`
	HttpPath    string           `json:"http_path,omitempty" yaml:"http_path,omitempty"`
	Type        string           `json:"type,omitempty" yaml:"type,omitempty"`
	Timeout     int              `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Concurrency int              `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
}

type PassiveHealthCheck struct {
	Healthy   *PassiveHealthyConfig   `json:"healthy,omitempty" yaml:"healthy,omitempty"`
	Unhealthy *PassiveUnhealthyConfig `json:"unhealthy,omitempty" yaml:"unhealthy,omitempty"`
	Type      string                  `json:"type,omitempty" yaml:"type,omitempty"`
}

type PassiveHealthyConfig struct {
	Successes int `json:"successes,omitempty" yaml:"successes,omitempty"`
}

type PassiveUnhealthyConfig struct {
	HttpFailures int `json:"http_failures,omitempty" yaml:"http_failures,omitempty"`
	TcpFailures  int `json:"tcp_failures,omitempty" yaml:"tcp_failures,omitempty"`
	Timeouts     int `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
}

type HealthyConfig struct {
	Interval  int `json:"interval,omitempty" yaml:"interval,omitempty"`
	Successes int `json:"successes,omitempty" yaml:"successes,omitempty"`
}

type UnhealthyConfig struct {
	Interval     int `json:"interval,omitempty" yaml:"interval,omitempty"`
	HttpFailures int `json:"http_failures,omitempty" yaml:"http_failures,omitempty"`
}

type UpstreamTarget struct {
	Id     string `json:"id" yaml:"id"`
	Target string `json:"target" yaml:"target"`
	Weight int    `json:"weight" yaml:"weight"`
}

type Health struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Path    string `json:"path" yaml:"path"`
}

type Route struct {
	Name    string            `json:"name" yaml:"name"`
	Path    string            `json:"path,omitempty" yaml:"path,omitempty"`       // Deprecated: use Paths
	Paths   []string          `json:"paths,omitempty" yaml:"paths,omitempty"`     // Kong-style: multiple paths per route
	Methods []string          `json:"methods,omitempty" yaml:"methods,omitempty"` // Optional: empty = all methods
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Plugins []PluginConfig    `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	// Backends for Response Aggregation (BFF pattern)
	// If specified, the gateway will call all backends in parallel and merge responses
	Backends []Backend `json:"backends,omitempty" yaml:"backends,omitempty"`
	// StripPath indicates if the matched path prefix should be removed before forwarding to upstream
	StripPath *bool `json:"strip_path,omitempty" yaml:"strip_path,omitempty"`
}

// Backend represents a single backend for response aggregation
type Backend struct {
	// Name is the key used in the merged JSON response
	Name string `json:"name" yaml:"name"`
	// Target is the backend service target (host:port)
	Target string `json:"target" yaml:"target"`
	// Path is the URL path to call on this backend (supports {param} placeholders)
	Path string `json:"path" yaml:"path"`
	// Computed/Linked fields
	// Protocol is http or https
	Protocol string `json:"protocol" yaml:"protocol"`
	// Timeout in milliseconds for this specific backend
	TimeoutMs int `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	// Required indicates if this backend must succeed for the overall request to succeed
	Required bool `json:"required" yaml:"required"`
	// ServiceName for dynamic discovery (optional overrides Host/Port)
	ServiceName string `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	// DiscoveryProvider (e.g., "consul", "etcd")
	DiscoveryProvider string `json:"discovery_provider,omitempty" yaml:"discovery_provider,omitempty"`
	// Timeouts
	ConnectTimeout int `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	ReadTimeout    int `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty"`
	WriteTimeout   int `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty"`
}

type Service struct {
	Name                  string                 `json:"name" yaml:"name"`
	URL                   string                 `json:"url,omitempty" yaml:"url,omitempty"` // Kong-style: single URL (http://host:port)
	ServiceName           string                 `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	DiscoveryProvider     string                 `json:"discovery_provider,omitempty" yaml:"discovery_provider,omitempty"`
	BasePath              string                 `json:"base_path,omitempty" yaml:"base_path,omitempty"`
	Protocol              string                 `json:"protocol,omitempty" yaml:"protocol,omitempty"` // Used if URL not specified
	Host                  string                 `json:"host,omitempty" yaml:"host,omitempty"`         // Refers to an Upstream name OR direct host
	LoadBalancingStrategy LoadBalancingAlgorithm `json:"load_balancing_strategy,omitempty" yaml:"load_balancing_strategy,omitempty"`
	Upstreams             []UpstreamTarget       `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`
	Plugins               []PluginConfig         `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Routes                []Route                `json:"routes,omitempty" yaml:"routes,omitempty"`
	Health                Health                 `json:"health,omitempty" yaml:"health,omitempty"`
	RetryOptions          RetryOptions           `json:"retry_options,omitempty" yaml:"retry_options,omitempty"`
	Retries               int                    `json:"retries,omitempty" yaml:"retries,omitempty"` // Kong-style simplified field
	ConnectTimeout        int                    `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	ReadTimeout           int                    `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty"`
	WriteTimeout          int                    `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty"`
	TLS                   UpstreamTLS            `json:"tls,omitempty" yaml:"tls,omitempty"`
	// Link to UpstreamConfig health checks
	UpstreamHealthChecks *HealthCheckConfig `json:"-" yaml:"-"`
}

type UpstreamTLS struct {
	Enabled    bool   `json:"enabled"`
	CaCertPath string `json:"ca_cert_path"`
	CertPath   string `json:"cert_path"`
	KeyPath    string `json:"key_path"`
}

type RetryOptions struct {
	MaxRetries      int `json:"max_retries"`
	RetryIntervalMs int `json:"retry_interval_ms"`
}
