package middleware

import (
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// UserExistsMiddleware Проверяет, что аутентифицированный пользователь
// существует в БД проекта. Если нет - возвращает 403.
func UserExistsMiddleware(storage storage.Storage) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(contextkeys.UserID).(string)
			if !ok || userID == "" {
				logger.Log.Error("Не удалось получить ID пользователя из контекста")
				response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
				return
			}

			exists, err := storage.UserExists(r.Context(), userID)
			if err != nil {
				// реальная ошибка БД/сети
				logger.Log.Error("Ошибка БД", logger.String("err", err.Error()))
				response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
				return
			}

			if !exists {
				// пользователь есть в контексте, но нет в БД проекта - рассинхрон
				logger.Log.Warn("Пользователь не найден в БД SWSM: возможен рассинхрон с БД Keycloak",
					logger.String("user_id", userID))
				response.ErrorJSON(w, http.StatusForbidden, "Пользователь не найден")
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}
