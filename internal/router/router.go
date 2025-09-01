package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router Роутер.
func Router() chi.Router {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hello, World!")) })

	return router
}
