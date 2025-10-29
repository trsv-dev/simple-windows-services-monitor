package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// ParseServerIDMiddleware извлекает и валидирует serverID из URL параметров роутера Chi.
func ParseServerIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "serverID")

		if idStr == "" {
			logger.Log.Error("В запросе отсутствует serverID")
			response.ErrorJSON(w, http.StatusBadRequest, "В запросе отсутствует id сервера")
			return
		}

		// Парсим строку в int64
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			logger.Log.Error("Некорректный id")
			response.ErrorJSON(w, http.StatusBadRequest, "Некорректный id сервера")
			return
		}

		if id <= 0 {
			logger.Log.Error("Некорректный id: должен быть положительным")
			response.ErrorJSON(w, http.StatusBadRequest, "id сервера должен быть положительным числом")
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.ServerID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ParseServiceIDMiddleware извлекает и валидирует serviceID из URL параметров роутера Chi.
func ParseServiceIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "serviceID")

		if idStr == "" {
			logger.Log.Error("В запросе отсутствует serviceID")
			response.ErrorJSON(w, http.StatusBadRequest, "В запросе отсутствует id службы")
			return
		}

		// Парсим строку в int64
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			logger.Log.Error("Некорректный id")
			response.ErrorJSON(w, http.StatusBadRequest, "Некорректный id службы")
			return
		}

		if id <= 0 {
			logger.Log.Error("Некорректный id: должен быть положительным")
			response.ErrorJSON(w, http.StatusBadRequest, "id службы должен быть положительным числом")
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.ServiceID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
