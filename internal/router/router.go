package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/middleware"
)

// Router Роутер.
func Router(h *api.AppHandler) chi.Router {
	router := chi.NewRouter()

	// middleware логгера
	router.Use(middleware.LogMiddleware)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hello, World!")) })
	return router
}
