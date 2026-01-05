package health_handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/worker"
)

// HealthHandler обрабатывает HTTP-запросы для проверки состояния сервиса.
type HealthHandler struct {
	storage   storage.Storage
	checker   netutils.Checker
	winrmPort string
}

// NewHealthHandler Конструктор HealthHandler.
func NewHealthHandler(storage storage.Storage, checker netutils.Checker, winrmPort string) *HealthHandler {
	return &HealthHandler{
		storage:   storage,
		checker:   checker,
		winrmPort: winrmPort,
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

func (h *HealthHandler) ServerStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	creds := models.GetContextCreds(ctx)

	server, err := h.storage.GetServer(ctx, creds.ServerID, creds.UserID)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		if errors.As(err, &ErrServerNotFound) {
			logger.Log.Warn("Сервер не найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		}

		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
		return
	}

	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	statusCtx, statusDone := context.WithTimeout(ctx, 3*time.Second)
	defer statusDone()

	statusCh := worker.ServerStatusWorker(statusCtx, h.checker, server, h.winrmPort)

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(<-statusCh); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
