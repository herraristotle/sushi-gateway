package container

import (
	"crypto/x509"
	"sync"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// AppConfig holds application-level configuration loaded from environment
type AppConfig struct {
	ServerCertPath  string
	ServerKeyPath   string
	CACertPath      string
	AdminUser       string
	AdminPassword   string
	AdminCorsOrigin string
	ConfigFilePath  string
	JwtSecret       []byte
}

// CaCertPool wraps the certificate pool for mTLS
type CaCertPool struct {
	Pool *x509.CertPool
}

// Container holds all shared dependencies for the application.
// This consolidates scattered global variables into a single struct
// for better organization and testability.
type Container struct {
	// Application configuration from environment
	AppConfig *AppConfig

	// Proxy configuration from config file
	ProxyConfig *model.ProxyConfig

	// Lock for thread-safe proxy config access
	ConfigLock *sync.RWMutex

	// CA Certificate pool for mTLS
	CaCertPool *CaCertPool

	// Health checker for upstream services
	HealthChecker HealthCheckerInterface

	// Circuit breakers map (per service)
	CircuitBreakers     map[string]*CircuitBreakerState
	CircuitBreakersLock *sync.RWMutex
}

// HealthCheckerInterface allows for mocking in tests
type HealthCheckerInterface interface {
	Initialize()
	CheckHealthForAllServices()
	IsUpstreamHealthy(serviceName string, upstreamIndex int) bool
}

// CircuitBreakerState tracks the state of each service's circuit
type CircuitBreakerState struct {
	State           int // 0=Closed, 1=Open, 2=HalfOpen
	Failures        int
	Successes       int
	LastFailureTime int64 // Unix timestamp
}

// Global is the single instance of Container
// This replaces multiple scattered globals
var Global *Container

// NewContainer creates a new dependency injection container
func NewContainer(appConfig *AppConfig) *Container {
	return &Container{
		AppConfig:           appConfig,
		ProxyConfig:         &model.ProxyConfig{},
		ConfigLock:          &sync.RWMutex{},
		CircuitBreakers:     make(map[string]*CircuitBreakerState),
		CircuitBreakersLock: &sync.RWMutex{},
	}
}

// GetProxyConfig returns the proxy config with read lock
func (c *Container) GetProxyConfig() model.ProxyConfig {
	c.ConfigLock.RLock()
	defer c.ConfigLock.RUnlock()
	return *c.ProxyConfig
}

// SetProxyConfig updates the proxy config with write lock
func (c *Container) SetProxyConfig(config *model.ProxyConfig) {
	c.ConfigLock.Lock()
	defer c.ConfigLock.Unlock()
	c.ProxyConfig = config
}

// Initialize sets up the global container instance
func Initialize(appConfig *AppConfig) {
	Global = NewContainer(appConfig)
}
