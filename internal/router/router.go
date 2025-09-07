package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/middleware"
)

// Router Роутер.
func Router(h *api.AppHandler) chi.Router {
	router := chi.NewRouter()

	// middleware логгера всех запросов
	router.Use(middleware.LogMiddleware)

	// публичные маршруты
	router.Post("/api/user/register", h.UserRegistration)
	router.Post("/api/user/login", h.UserAuthorization)

	// маршруты, требующие авторизацию
	router.Route("/api/user", func(r chi.Router) {

		// middleware для всех приватных маршрутов
		r.Use(middleware.LoginToContextMiddleware)
		r.Use(middleware.RequireAuthMiddleware)

		// маршруты БЕЗ ID параметра
		r.Post("/servers", h.AddServer)    // создание сервера
		r.Get("/servers", h.GetServerList) // список серверов пользователя

		// маршруты С ID параметром
		r.Route("/servers/{id}", func(r chi.Router) {

			// извлекаем ID из параметров роутера
			r.Use(middleware.ParseIDMiddleware)

			r.Patch("/", h.EditServer)        // редактирование сервера
			r.Delete("/", h.DelServer)        // удаление сервера
			r.Get("/", h.GetServer)           // получение сервера
			r.Post("/services", h.AddService) // добавление службы
		})
	})

	return router
}
