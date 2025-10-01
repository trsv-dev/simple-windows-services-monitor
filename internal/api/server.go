package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
)

// AddServer Добавление нового сервера.
func (h *AppHandler) AddServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	var server models.Server

	err := json.NewDecoder(r.Body).Decode(&server)
	if err != nil {
		logger.Log.Debug("Неверный формат запроса для добавления сервера", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err := server.CreateValidation(); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	fingerprint, err := service_control.GetFingerprint(ctx, server.Address, server.Username, server.Password)
	if err != nil {
		logger.Log.Error("Ошибка получения уникального идентификатора сервера", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Ошибка получения уникального идентификатора сервера"))
		return
	}

	server.Fingerprint = fingerprint

	createdServer, err := h.storage.AddServer(ctx, server, creds.UserID)

	var ErrDuplicatedServer *errs.ErrDuplicatedServer
	if err != nil {
		switch {
		case errors.As(err, &ErrDuplicatedServer):
			logger.Log.Error("Дубликат сервера", logger.String("err", ErrDuplicatedServer.Err.Error()))
			response.ErrorJSON(w, http.StatusConflict, "Сервер уже был добавлен")
			return
		default:
			logger.Log.Error("Ошибка добавления сервера в БД", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка добавления сервера")
			return
		}
	}

	logger.Log.Debug("Сервер успешно добавлен пользователем",
		logger.String("login", creds.Login), logger.String("address", server.Address))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err = json.NewEncoder(w).Encode(createdServer); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// EditServer Редактирование пользовательского сервера.
func (h *AppHandler) EditServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	// получаем текущие данные сервера
	old, err := h.storage.GetServerWithPassword(ctx, creds.ServerID, creds.UserID)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Error("Сервер не был найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не был найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	// читаем данные из входящего JSON с обновленной информацией о сервере
	var input models.Server

	if err = json.NewDecoder(r.Body).Decode(&input); err != nil {
		logger.Log.Debug("Неверный формат запроса для редактирования сервера", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err = input.UpdateValidation(); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// обновляем полученными данными текущий сервер
	if input.Name != "" {
		old.Name = input.Name
	}

	if input.Username != "" {
		old.Username = input.Username
	}

	if input.Password != "" {
		old.Password = input.Password
	}

	if input.Address != "" {
		password := old.Password
		if input.Password != "" {
			password = input.Password
		}

		fingerprint, err := service_control.GetFingerprint(ctx, input.Address, input.Username, password)
		if err != nil {
			logger.Log.Error("Ошибка получения уникального идентификатора сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Ошибка получения уникального идентификатора сервера"))
			return
		}
		if old.Fingerprint != fingerprint {
			logger.Log.Error("Невозможно изменить адрес: UUID сервера не совпадает с ранее зарегистрированным UUID", logger.String("newAddress", input.Address), logger.String("oldAddress", old.Address))
			response.ErrorJSON(w, http.StatusBadRequest, fmt.Sprintf("Невозможно изменить адрес: UUID сервера `%s` не совпадает с ранее зарегистрированным UUID `%s`", input.Address, old.Address))
			return
		}

		old.Address = input.Address
	}

	// производим обновление сервера в БД
	editedServer, err := h.storage.EditServer(ctx, old, creds.ServerID, creds.UserID)
	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при обновлении сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, "Ошибка при обновлении сервера")
			return
		}
	}

	logger.Log.Debug("Сервер успешно отредактирован пользователем", logger.String("login", creds.Login),
		logger.Int64("serverID", creds.ServerID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(editedServer); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// DelServer Удаление сервера, добавленного пользователем.
func (h *AppHandler) DelServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	err := h.storage.DelServer(ctx, creds.ServerID, creds.UserID)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		case err != nil:
			logger.Log.Warn("Ошибка при удалении сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, "Ошибка при удалении сервера")
			return
		}
	}

	logger.Log.Debug("Сервер успешно удален пользователем", logger.String("login", creds.Login),
		logger.Int64("serverID", creds.ServerID))
	//response.SuccessJSON(w, http.StatusAccepted, "Сервер успешно удален")
	w.WriteHeader(http.StatusNoContent)
}

// GetServer Получение информации о сервере.
func (h *AppHandler) GetServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	server, err := h.storage.GetServer(ctx, creds.ServerID, creds.UserID)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
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
	ctx := r.Context()
	userID := ctx.Value(contextkeys.ID).(int64)

	servers, err := h.storage.ListServers(ctx, userID)
	if err != nil {
		logger.Log.Warn("Ошибка при получении списка серверов пользователя", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении списка серверов")
		return
	}

	// если серверов у пользователя нет - возвращаем пустой срез серверов
	if len(servers) == 0 {
		servers = []*models.Server{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(servers); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
