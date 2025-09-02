package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
)

// Router Роутер.
func Router(h *api.AppHandler) chi.Router {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hello, World!")) })
	return router
}
