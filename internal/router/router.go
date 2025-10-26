package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/middleware"
)

// Router Роутер.
func Router(h *api.HandlersContainer) chi.Router {
	router := chi.NewRouter()

	router.Use(middleware.CorsMiddleware)

	// middleware логгера всех запросов
	router.Use(middleware.LogMiddleware)

	// публичные маршруты
	router.Post("/api/user/register", h.RegistrationHandler.UserRegistration)
	router.Post("/api/user/login", h.AuthorizationHandler.UserAuthorization)

	// маршруты, требующие авторизацию
	router.Route("/api/user", func(r chi.Router) {

		// middleware для всех приватных маршрутов
		r.Use(middleware.UserLoginUserIdToContextMiddleware(h.AppHandler.JWTSecretKey))
		r.Use(middleware.RequireAuthMiddleware)

		// SSE: подписка на события служб
		// h.Broadcaster.HTTPHandler() — это http.Handler для всех топиков
		r.Handle("/broadcasting", h.AppHandler.Broadcaster.HTTPHandler())

		// маршруты БЕЗ ServerID параметра
		r.Post("/servers", h.ServerHandler.AddServer)    // создание сервера
		r.Get("/servers", h.ServerHandler.GetServerList) // список серверов пользователя

		// маршруты С serverID параметром
		r.Route("/servers/{serverID}", func(r chi.Router) {

			// извлекаем serverID из параметров роутера
			r.Use(middleware.ParseServerIDMiddleware)

			r.Patch("/", h.ServerHandler.EditServer) // редактирование сервера
			r.Delete("/", h.ServerHandler.DelServer) // удаление сервера
			r.Get("/", h.ServerHandler.GetServer)    // получение сервера

			r.Route("/services", func(r chi.Router) {
				r.Post("/", h.ServiceHandler.AddService)     // добавление службы
				r.Get("/", h.ServiceHandler.GetServicesList) // список служб сервера

				// маршруты С serviceID параметром
				r.Route("/{serviceID}", func(r chi.Router) {

					// извлекаем serviceID из параметров роутера
					r.Use(middleware.ParseServiceIDMiddleware)

					r.Delete("/", h.ServiceHandler.DelService) //удаление службы
					r.Get("/", h.ServiceHandler.GetService)    // получение службы

					// управление службами
					r.Post("/start", h.ControlHandler.ServiceStart)     // запуск службы
					r.Post("/stop", h.ControlHandler.ServiceStop)       // остановка службы
					r.Post("/restart", h.ControlHandler.ServiceRestart) // перезапуск службы
				})
			})
		})
	})

	return router
}
