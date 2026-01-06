package gateway

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
)

func NewRouter() *chi.Mux {
	slog.Info("Creating new router...")
	router := chi.NewRouter()

	egressController := NewSushiProxy()
	egressController.RegisterRoutes(router)

	slog.Info("Successfully created router...")
	return router
}
