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

// ParseIDMiddleware извлекает и валидирует ID из URL параметров роутера Chi.
func ParseIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")

		if idStr == "" {
			logger.Log.Error("В запросе отсутствует ID")
			response.ErrorJSON(w, http.StatusBadRequest, "В запросе отсутствует ID")
			return
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Log.Error("Некорректный id")
			response.ErrorJSON(w, http.StatusBadRequest, "Некорректный id")
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.IDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
