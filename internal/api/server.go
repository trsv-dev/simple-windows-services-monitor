package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/utils"
)

// AddServer Добавление нового сервера.
func (h *AppHandler) AddServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	login, ok := ctx.Value(contextkeys.Login).(string)
	if !ok || login == "" {
		logger.Log.Error("Не удалось получить логин из контекста")
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	userID, err := h.storage.GetUserIDByLogin(ctx, login)
	if err != nil {
		logger.Log.Error("Пользователь не найден", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusUnauthorized, "Пользователь не найден")
		return
	}

	var server models.Server

	err = json.NewDecoder(r.Body).Decode(&server)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if !utils.IsValidIP(server.Address) {
		response.ErrorJSON(w, http.StatusBadRequest, "Невалидный IP адрес сервера")
		return
	}

	err = h.storage.AddServer(ctx, server, userID)

	var ErrDuplicatedServer *errs.ErrDuplicatedServer
	if err != nil {
		switch {
		case errors.As(err, &ErrDuplicatedServer):
			logger.Log.Error("Дубликат сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Сервер уже был добавлен")
			return
		default:
			logger.Log.Error("Ошибка добавления сервера в БД", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка добавления сервера")
			return
		}
	}

	logger.Log.Debug("Сервер успешно добавлен пользователем", logger.String("login", login), logger.String("address", server.Address))
	response.SuccessJSON(w, http.StatusOK, "Сервер успешно добавлен")
}

// DelServer Удаление сервера, добавленного пользователем.
func (h *AppHandler) DelServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	login, ok := ctx.Value(contextkeys.Login).(string)
	if !ok || login == "" {
		logger.Log.Error("Не удалось получить логин из контекста")
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	var req struct {
		SrvAddr string `json:"address"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if req.SrvAddr == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "Адрес сервера не указан")
		return
	}

	if !utils.IsValidIP(req.SrvAddr) {
		response.ErrorJSON(w, http.StatusBadRequest, "Невалидный IP адрес сервера")
		return
	}

	err = h.storage.DelServer(ctx, req.SrvAddr, login)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден", logger.String("login", ErrServerNotFound.Login),
				logger.String("address", ErrServerNotFound.Address), logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, "Сервер не найден")
			return
		case err != nil:
			logger.Log.Warn("Ошибка при удалении сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, "Ошибка при удалении сервера")
			return
		}
	}

	logger.Log.Warn("Сервер успешно удален пользователем", logger.String("login", login),
		logger.String("address", req.SrvAddr))
	response.SuccessJSON(w, http.StatusAccepted, "Сервер успешно удален")
}

// GetServer Получение информации о сервере.
func (h *AppHandler) GetServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	login, ok := ctx.Value(contextkeys.Login).(string)
	if !ok || login == "" {
		logger.Log.Error("Не удалось получить логин из контекста")
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	var req struct {
		SrvAddr string `json:"address"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if !utils.IsValidIP(req.SrvAddr) {
		response.ErrorJSON(w, http.StatusBadRequest, "Невалидный IP адрес сервера")
		return
	}

	server, err := h.storage.GetServer(ctx, req.SrvAddr, login)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден", logger.String("login", ErrServerNotFound.Login),
				logger.String("address", ErrServerNotFound.Address), logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(server); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// GetServerList Получение списка серверов пользователя.
func (h *AppHandler) GetServerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	login, ok := ctx.Value(contextkeys.Login).(string)
	if !ok || login == "" {
		logger.Log.Error("Не удалось получить логин из контекста")
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	servers, err := h.storage.ListServers(ctx, login)
	if err != nil {
		logger.Log.Warn("Ошибка при получении списка серверов пользователя", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении списка серверов")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(servers); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
