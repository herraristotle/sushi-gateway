package gateway

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/discovery/consul"
)

// LoadGlobalConfig loads configuration from environment and initializes the DI container.
// Returns the container.AppConfig for use during startup.
func LoadGlobalConfig() (*container.AppConfig, error) {
	slog.Info("Loading Global application config for sushi gateway from environment variables...")
	godotenv.Load()

	errors := make([]string, 0)

	// Get certificate paths from environment
	serverCertPath := os.Getenv("SERVER_CERT_PATH")
	serverKeyPath := os.Getenv("SERVER_KEY_PATH")

	// Check if server cert paths are provided
	hasServerCert := serverCertPath != ""
	hasServerKey := serverKeyPath != ""

	// If any server cert path is provided, both must be provided
	if hasServerCert || hasServerKey {
		// When both server cert and key is provided, it is loaded into our configuration,
		// else it is an user configuration error as they did not pass either cert or key
		if !hasServerCert {
			errors = append(errors, "if you want to use your own certificates, SERVER_CERT_PATH is required when SERVER_KEY_PATH is provided. for auto generating the certificates, leave both SERVER_CERT_PATH and SERVER_KEY_PATH empty")
		}
		if !hasServerKey {
			errors = append(errors, "if you want to use your own certificates, SERVER_KEY_PATH is required when SERVER_CERT_PATH is provided. for auto generating the certificates, leave both SERVER_CERT_PATH and SERVER_KEY_PATH empty")
		}
	} else {
		// If user did not spsecify both the server cert or key, we generate the self signed cert in the gateway on load.
		slog.Info("Since no certs were found, auto generating self signed certs for the TLS server...")
		if err := GenerateSelfSignedCerts("."); err != nil {
			slog.Error("Failed to generate self-signed certificates", "error", err)
			errors = append(errors, "Failed to generate self-signed certificates")
		} else {
			// Set certificate paths
			serverCertPath = filepath.Join(".", "server.crt")
			serverKeyPath = filepath.Join(".", "server.key")
		}
	}

	// Optional, we only need CA Certs for MTLS communications
	caCertPath := os.Getenv("CA_CERT_PATH")

	// CORS configurations

	// Admin User and Password is used for ADMIN API credentials
	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		errors = append(errors, "ADMIN_USER is required.")
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		errors = append(errors, "ADMIN_PASSWORD is required.")
	}

	// Admin API Cors configurations, optional, if not provided, set to default localhost sushi manager (localhost:5173)
	adminCorsOrigin := os.Getenv("ADMIN_CORS_ORIGIN")

	// Defines the path to our declarative configuration file to load configurations for the gateway.
	configFilePath := os.Getenv("CONFIG_FILE_PATH")
	if configFilePath == "" {
		errors = append(errors, "CONFIG_FILE_PATH is required.")
	}

	// Redis configuration (required for distributed rate limiting and caching)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		errors = append(errors, "REDIS_ADDR is required for distributed rate limiting and caching.")
	}
	redisPassword := os.Getenv("REDIS_PASSWORD") // Optional, default empty
	redisDB := 0                                 // Default to DB 0
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "sushi-gateway.db" // Default sqlite file
	}

	// JWT secret for Admin API authentication
	// If not provided, auto-generate a secure random secret
	var jwtSecret []byte
	jwtSecretEnv := os.Getenv("JWT_SECRET")
	if jwtSecretEnv != "" {
		jwtSecret = []byte(jwtSecretEnv)
		slog.Info("Using JWT_SECRET from environment variable")
	} else {
		// Auto-generate a secure 32-byte random secret
		jwtSecret = make([]byte, 32)
		if _, err := rand.Read(jwtSecret); err != nil {
			errors = append(errors, "Failed to generate JWT secret: "+err.Error())
		} else {
			slog.Warn("JWT_SECRET not set, auto-generated a random secret. Sessions will be invalidated on restart.")
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			slog.Error(err)
		}
		slog.Error("Errors detected when loading environment configuration exiting...")
		return nil, fmt.Errorf("failed to load environment configuration")
	}

	config := &container.AppConfig{
		ServerCertPath:  serverCertPath,
		ServerKeyPath:   serverKeyPath,
		CACertPath:      caCertPath,
		AdminUser:       adminUser,
		AdminPassword:   adminPassword,
		AdminCorsOrigin: adminCorsOrigin,
		ConfigFilePath:  configFilePath,
		JwtSecret:       jwtSecret,
		RedisAddr:       redisAddr,
		RedisPassword:   redisPassword,
		RedisDB:         redisDB,
		DbPath:          dbPath,
	}

	// Initialize the DI container
	cont := container.NewContainer(config)

	// Initialize Service Discovery Registry (Consul)
	consulAddr := os.Getenv("CONSUL_ADDRESS")
	if consulAddr != "" {
		slog.Info("Initializing Consul Registry", "address", consulAddr)
		registry, err := consul.NewConsulRegistry(consulAddr)
		if err != nil {
			slog.Error("Failed to initialize Consul registry", "error", err)
			return nil, err
		}
		cont.Registry = registry
	} else {
		// Try localhost default if not specified but maybe needed?
		// For now, only if CONSUL_ADDRESS is set.
		// Or we can default to localhost:8500 if user wants default?
		// Let's stick to explicit env var for now.
	}

	container.Global = cont
	slog.Info("Initialized DI container")

	return config, nil
}
