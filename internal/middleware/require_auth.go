package middleware

import (
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// RequireAuthMiddleware проверяет наличие ID пользователя в контексте.
func RequireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(contextkeys.UserID).(string)
		if !ok || userID == "" {
			logger.Log.Error("Не удалось получить ID пользователя из контекста")
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
			return
		}

		next.ServeHTTP(w, r)
	})
}
