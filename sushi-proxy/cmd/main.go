package main

import (
	"context"
	"crypto/tls"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/api"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/gateway"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/errgroup"
)

func main() {
	// Initialize structured logging (JSON) for production
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	var shutdownTracer func(context.Context) error
	var err error

	// Load gateway environment config (this also initializes container.Global)
	_, err = gateway.LoadGlobalConfig()
	if err != nil {
		os.Exit(1)
	}

	// Initialize SQL Store for dynamic configuration
	if err := gateway.InitSQLStore(container.Global.AppConfig.DbPath); err != nil {
		slog.Error("Failed to initialize SQL store", "error", err)
		os.Exit(1)
	}

	// Initialize Redis client (required for distributed rate limiting and caching)
	err = gateway.InitRedisClient(gateway.RedisConfig{
		Addr:     container.Global.AppConfig.RedisAddr,
		Password: container.Global.AppConfig.RedisPassword,
		DB:       container.Global.AppConfig.RedisDB,
	})
	if err != nil {
		slog.Error("Failed to initialize Redis", "error", err)
		os.Exit(1)
	}
	defer gateway.CloseRedisClient()

	// Setup error group with cancellation context
	errGrpCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errGroup, errGrpCtx := errgroup.WithContext(errGrpCtx)

	// Initialize all servers and routers first
	appRouter := gateway.NewRouter()

	// Wrap router with OpenTelemetry Middleware if enabled
	var finalHandler http.Handler = appRouter
	if gateway.EnableOTEL {
		finalHandler = otelhttp.NewHandler(appRouter, "sushi-gateway-handler")
	}

	// Initialize HTTP server
	httpServer := &http.Server{
		Addr:    ":" + constant.PORT_HTTP,
		Handler: finalHandler,
	}

	// Initialize HTTPS server
	cert, err := tls.LoadX509KeyPair(container.Global.AppConfig.ServerCertPath, container.Global.AppConfig.ServerKeyPath)
	if err != nil {
		slog.Error("Failed to load TLS keys", "error", err)
		log.Fatal(err)
	}

	// Load global CA Cert Pool, allowing clients to send CA certificate for authentication via MTLS
	gateway.GlobalCaCertPool = gateway.LoadCertPool()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    gateway.GlobalCaCertPool.Pool,
		ClientAuth:   tls.RequestClientCert,
	}

	httpsServer := &http.Server{
		Addr:      ":" + constant.PORT_HTTPS,
		Handler:   finalHandler,
		TLSConfig: tlsConfig,
	}

	// Initialize Admin API server
	var adminApiRouter http.Handler

	// Create router for admin API
	adminApiRouter = api.NewAdminApiRouter()

	adminServer := &http.Server{
		Addr:    ":" + constant.PORT_ADMIN_API,
		Handler: adminApiRouter,
	}

	// Initialize config file watcher
	// Do this on gateway startup, load the config from config
	if err := gateway.LoadProxyConfigFromConfigFile(container.Global.AppConfig.ConfigFilePath); err != nil {
		slog.Error("Failed to load initial config file", "error", err)
		os.Exit(1)
	}

	// Initialize OpenTelemetry if enabled
	if gateway.EnableOTEL {
		shutdownTracer, err = telemetry.InitTracer("sushi-gateway")
		if err != nil {
			slog.Error("Failed to initialize OpenTelemetry", "error", err)
		}
		defer func() {
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		}()
	}

	// Start the file watcher
	errGroup.Go(func() error {
		return gateway.WatchConfigFile(errGrpCtx, container.Global.AppConfig.ConfigFilePath)
	})

	// Start health checker, we start the health checker before the servers start, so that we can verify the health of the services before they are proxied.
	// We also add it to the error group, so that it can be stopped gracefully when the gateway is shutdown.
	gateway.GlobalHealthChecker.Initialize()
	gateway.GlobalHealthChecker.CheckHealthForAllServices() // Initial health check, run it once before starting the ticker
	errGroup.Go(func() error {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Periodic health checks
		for {
			select {
			case <-errGrpCtx.Done():
				slog.Info("Stopping health checker...")
				return nil // Stop the health checker by exiting the infinite loop
			case <-ticker.C:
				gateway.GlobalHealthChecker.CheckHealthForAllServices()
			}
		}
	})

	// Start all servers concurrently
	// Start HTTP server
	errGroup.Go(func() error {
		slog.Info("Started sushi-proxy_pass http server on port: " + constant.PORT_HTTP)

		// Graceful shutdown on context cancellation
		go func() {
			<-errGrpCtx.Done()
			httpServer.Shutdown(context.Background())
			slog.Info("Gracefully shutdown HTTP Server....")
		}()

		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			return err
		}
		return nil
	})

	// Start HTTPS server
	errGroup.Go(func() error {
		slog.Info("Started sushi-proxy_pass https server on port: " + constant.PORT_HTTPS)

		// Graceful shutdown on context cancellation
		go func() {
			<-errGrpCtx.Done()
			httpsServer.Shutdown(context.Background())
			slog.Info("Gracefully shutdown HTTPS Server....")
		}()

		if err := httpsServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			slog.Error("HTTPS server failed", "error", err)
			return err
		}
		return nil
	})

	// Start Admin API server
	errGroup.Go(func() error {
		slog.Info("Started admin API server on port: " + constant.PORT_ADMIN_API)

		// Graceful shutdown on context cancellation
		go func() {
			<-errGrpCtx.Done()
			adminServer.Shutdown(context.Background())
			slog.Info("Gracefully shutdown Admin API Server....")
		}()

		if err := adminServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Admin API server failed", "error", err)
			return err
		}
		return nil
	})

	// Wait for all servers and handle errors
	if err := errGroup.Wait(); err != nil {
		slog.Error("Server error detected, shutting down...", "error", err)
		log.Fatal(err)
	}
}
