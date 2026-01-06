package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type HealthController struct {
}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (c *HealthController) RegisterRoutes(router chi.Router) {
	router.Get("/healthz", c.CheckHealth())
}

func (c *HealthController) CheckHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("HealthController:: Admin API - Gateway is healthy.")
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}
}
