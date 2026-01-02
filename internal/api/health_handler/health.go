package health_handler

import (
	"context"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// HealthHandler обрабатывает HTTP-запросы для проверки состояния сервиса.
type HealthHandler struct {
	storage storage.Storage
}

// NewHealthHandler Конструктор HealthHandler.
func NewHealthHandler(storage storage.Storage) *HealthHandler {
	return &HealthHandler{
		storage: storage,
	}
}

// GetHealth обрабатывает health_handler-check запрос и возвращает статус готовности сервиса.
// Возвращает HTTP 200, если база данных доступна, иначе HTTP 503.
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
	defer pingCancel()

	if err := h.storage.Ping(pingCtx); err != nil {
		logger.Log.Error("База данных PostgreSQL не отвечает", logger.String("error", err.Error()))

		http.Error(w, "База данных недоступна", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
