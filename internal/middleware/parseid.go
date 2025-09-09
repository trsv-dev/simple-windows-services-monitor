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

		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Log.Error("Некорректный id")
			response.ErrorJSON(w, http.StatusBadRequest, "Некорректный id сервера")
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

		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Log.Error("Некорректный id")
			response.ErrorJSON(w, http.StatusBadRequest, "Некорректный id службы")
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.ServiceID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
