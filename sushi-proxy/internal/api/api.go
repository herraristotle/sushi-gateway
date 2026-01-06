package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/gateway"
	"github.com/rs/cors"
)

const DEFAULT_CORS_ORIGIN = "http://localhost:5173"

func NewAdminApiRouter() http.Handler {
	slog.Info("Creating new admin api router...")
	router := chi.NewRouter()
	router.Use(gateway.GlobalConnectionTracker.Track)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	gatewayController := NewGatewayController()
	gatewayController.RegisterRoutes(router)

	authController := NewAuthController()
	authController.RegisterRoutes(router)

	healthController := NewHealthController()
	healthController.RegisterRoutes(router)

	statsController := NewStatsController()
	statsController.RegisterRoutes(router)

	// Prometheus metrics endpoint
	router.Handle("/metrics", promhttp.Handler())
	slog.Info("Registered /metrics endpoint for Prometheus")

	corsOrigin := container.Global.AppConfig.AdminCorsOrigin
	if corsOrigin == "" {
		corsOrigin = DEFAULT_CORS_ORIGIN
	}

	corsRouter := cors.New(cors.Options{
		AllowedOrigins:   []string{corsOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	slog.Info("Successfully created admin api router...")
	return corsRouter.Handler(router)
}
