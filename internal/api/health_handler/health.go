package health_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// HealthHandler обрабатывает HTTP-запросы для проверки состояния сервиса.
type HealthHandler struct {
	storage     storage.Storage
	statusCache health_storage.StatusCacheStorage
	checker     netutils.Checker
}

// NewHealthHandler Конструктор HealthHandler.
func NewHealthHandler(storage storage.Storage, statusCache health_storage.StatusCacheStorage, checker netutils.Checker) *HealthHandler {
	return &HealthHandler{
		storage:     storage,
		statusCache: statusCache,
		checker:     checker,
	}
}

// GetHealth Обрабатывает health_handler-check запрос и возвращает статус готовности сервиса.
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

// ServerStatus Возвращает статус сервера, получая его из кэша health_storage.StatusCache.
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

	// получаем статус сервера из кэша, в который их пишет воркер ServerStatusWorker
	status, ok := h.statusCache.Get(server.ID)

	if !ok {
		logger.Log.Error(fmt.Sprintf("Статус сервера с ID=%d, Address=%s не найден", server.ID, server.Address))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(status); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// ServersStatuses Возвращает массив статусов серверов пользователя, получая его из кэша health_storage.StatusCache.
func (h *HealthHandler) ServersStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	creds := models.GetContextCreds(ctx)

	servers, err := h.storage.ListServers(ctx, creds.UserID)

	// если серверов у пользователя нет - возвращаем пустой срез серверов
	if len(servers) == 0 {
		servers = []*models.Server{}
	}

	// получаем статусы серверов из кэша, в который их пишет воркер ServerStatusWorker
	statuses := make([]models.ServerStatus, 0, len(servers))

	for _, server := range servers {
		status, ok := h.statusCache.Get(server.ID)
		if !ok {
			logger.Log.Error(fmt.Sprintf("Статус сервера с ID=%d, Address=%s не найден", server.ID, server.Address))
			response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			continue
		}

		statuses = append(statuses, status)
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(statuses); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
