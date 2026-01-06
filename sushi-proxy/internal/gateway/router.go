package gateway

import (
	"log/slog"

	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() *chi.Mux {
	slog.Info("Creating new router...")
	router := chi.NewRouter()
	router.Use(GlobalConnectionTracker.Track)

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	egressController := NewSushiProxy()
	egressController.RegisterRoutes(router)

	slog.Info("Successfully created router...")
	return router
}
