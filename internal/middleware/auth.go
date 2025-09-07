package middleware

import (
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// RequireAuthMiddleware проверяет наличие логина в контексте.
func RequireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login, ok := r.Context().Value(contextkeys.Login).(string)
		if !ok || login == "" {
			logger.Log.Error("Не удалось получить логин из контекста")
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
			return
		}

		next.ServeHTTP(w, r)
	})
}
