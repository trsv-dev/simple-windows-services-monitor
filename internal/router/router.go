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

	// middleware логгера всех запросов
	router.Use(middleware.LogMiddleware)

	// публичные маршруты:

	// Hello, World!
	router.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hello, World!")) })

	// регистрация нового пользователя
	router.Post("/api/user/register", h.UserRegistration)
	// авторизация пользователя (логин)
	router.Post("/api/user/login", h.UserAuthorization)

	// маршруты, требующие авторизацию:
	router.Route("/api/user", func(r chi.Router) {
		// middleware добавления логина зарегистрированного и авторизованного пользователя в контекст
		r.Use(middleware.LoginToContextMiddleware)
	})

	return router
}
